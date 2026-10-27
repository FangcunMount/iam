//go:build integration

package migration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	assignmentapp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/assignment"
	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/management"
	grantapp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/permissiongrant"
	roleapp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/role"
	appuow "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/uow"
	authzcontainer "github.com/FangcunMount/iam/v5/internal/apiserver/container/authz"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/subject"
	grantrepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/permissiongrant"
	policyrepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/policy"
	resourcerepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/resource"
	rolerepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/role"
	host "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/authz"
	transportrest "github.com/FangcunMount/iam/v5/internal/apiserver/transport/rest"
	authzhttp "github.com/FangcunMount/iam/v5/internal/apiserver/transport/rest/authz"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/FangcunMount/iam/v5/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/reliable-messaging/transaction"
	"github.com/FangcunMount/reliable-messaging/transport"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type readFixtureStager struct{}

func (readFixtureStager) Stage(context.Context, ...event.DomainEvent) error { return nil }

func seedReadAssignment(t *testing.T, db *gorm.DB, base appuow.UnitOfWork, roleID, userID meta.ID) {
	t.Helper()
	roles := rolerepo.NewRoleRepository(db)
	commands := assignmentapp.NewCommandService(assignment.NewValidator(roles, nativeSubjects{}), roles, base, nil, management.NewGuard(nil))
	_, err := commands.Grant(management.WithAuthenticatedService(t.Context(), "admin"), assignmentapp.GrantCommand{
		SubjectType: assignment.SubjectTypeUser, SubjectID: userID, RoleID: roleID, GrantedBy: "owned-read-fixture",
	})
	require.NoError(t, err)
}

// Uses the real module composition, router, services, planned UOW and MySQL.
// Authentication and Broker I/O are fixtures; this is not a production login
// or Broker end-to-end test. GET must never reach the stored-half sender.
func testRoleDetailsReadNative(t *testing.T, db *gorm.DB, binding transaction.Binding) {
	local, err := host.NewTransactionUnitOfWork(db, nativeSubjects{}, binding)
	require.NoError(t, err)
	sender := &nativeStoredHalfSender{store: local.Store()}
	topic := "iam.authz.version.v2"
	physical, err := transport.PhysicalTopicV2(topic, transport.TransactionMessageType, binding.Owner, "m3-1")
	require.NoError(t, err)
	writer, err := host.NewPlannedUnitOfWork(local, sender, host.PlanConfig{
		LogicalTopic: topic, PhysicalTopic: physical, Sends: 8, ProofBudget: time.Second,
		Admission: func() bool { return false }, // reads remain available while writes are paused
	})
	require.NoError(t, err)
	called := false
	require.ErrorIs(t, writer.WithinTx(t.Context(), func(context.Context, appuow.TxRepositories) error {
		called = true
		return nil
	}), appuow.ErrWritePlanRequired)
	require.False(t, called, "legacy write entry must remain closed")

	// Seed facts using the existing unplanned fixture boundary, before reads.
	admin := management.WithAuthenticatedService(t.Context(), "admin")
	base := host.NewUnitOfWork(db, nativeSubjects{}, readFixtureStager{})
	roles := roleapp.NewRoleCatalog(base, nil, management.NewGuard(nil))
	createRole := func(name string) meta.ID {
		cmd, err := roleapp.NewCreateRoleCommand(name, name, "")
		require.NoError(t, err)
		cmd.ChangedBy = "owned-read-fixture"
		r, err := roles.CreateRole(admin, cmd)
		require.NoError(t, err)
		return r.ID
	}
	emptyID := createRole("test:details_empty")
	populatedID := createRole("test:details_populated")
	readerID := createRole("test:details_reader")
	grants := grantapp.NewService(base, grantrepo.NewRepository(db), nil, management.NewGuard(nil))
	resources := resourcerepo.NewResourceRepository(db)
	for _, key := range []string{"iam:authz:collection:permission_grants", "iam:authz:collection:assignments"} {
		resource, err := resources.FindByKey(admin, key)
		require.NoError(t, err)
		_, err = grants.Create(admin, grantapp.CreateCommand{RoleID: readerID, ResourceID: resource.ID, Action: "list", GrantedBy: "owned-read-fixture"})
		require.NoError(t, err)
	}
	resource, err := resources.FindByKey(admin, "iam:authz:collection:permission_grants")
	require.NoError(t, err)
	grant, err := grants.Create(admin, grantapp.CreateCommand{RoleID: populatedID, ResourceID: resource.ID, Action: "list", GrantedBy: "owned-read-fixture"})
	require.NoError(t, err)
	// Reuse the original assignment helper and trusted subject fixture.
	seedReadAssignment(t, db, base, readerID, meta.ID(7))
	seedReadAssignment(t, db, base, populatedID, meta.ID(7))

	module := authzcontainer.NewAuthzModule()
	require.NoError(t, module.InitializeWithDeps(authzcontainer.AuthzModuleDeps{
		DB: db, EventStager: writer, UserResolver: fanoutUserResolver{},
		AssignmentConstraintsFile: "../../../configs/grpc_assignment_constraints.yaml",
	}))
	_, err = module.ApplicationCapabilities().PermissionGrantService.ListByRole(admin, emptyID)
	require.NoError(t, err, "real module must expose the planned UOW read boundary")
	deps := transportrest.Deps{}
	authzcontainer.CollectREST(true, module, &deps)
	api := authzhttp.Dependencies{
		RoleHandler: deps.Authz.RoleHandler, AssignmentHandler: deps.Authz.AssignmentHandler,
		PermissionGrantHandler: deps.Authz.PermissionGrantHandler,
	}
	actor := meta.ID(10001) // original migrated system role assignment
	guard := management.NewGuard(module.ApplicationCapabilities().AuthorizationDecisions)
	api.AuthMiddleware = func(c *gin.Context) {
		sub, err := subject.NewUserRef(actor)
		require.NoError(t, err)
		requestctx.SetUserID(c, actor)
		c.Request = c.Request.WithContext(management.WithAuthenticatedUser(c.Request.Context(), sub))
		c.Next()
	}
	api.Permission = func(resource, action string) gin.HandlerFunc {
		return func(c *gin.Context) {
			if err := guard.RequireOperation(c.Request.Context(), resource, action); err != nil {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
		}
	}
	engine := gin.New()
	authzhttp.Register(engine, api)
	request := func(id meta.ID, endpoint string, status, count int) {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, "/api/v4/authz/roles/"+id.String()+"/"+endpoint, nil)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		require.Equal(t, status, w.Code, w.Body.String())
		require.Empty(t, w.Header().Get("Operation-ID"))
		if status == http.StatusOK {
			var body struct {
				Code int               `json:"code"`
				Data []json.RawMessage `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Equal(t, 200, body.Code)
			require.Len(t, body.Data, count)
			if count == 0 {
				require.NotNil(t, body.Data, "empty response must be []")
			}
			if endpoint == "grants" && id == populatedID {
				require.Contains(t, string(body.Data[0]), grant.ID.String())
			}
		}
	}
	counts := func() map[string]int64 {
		result := make(map[string]int64)
		for _, table := range []string{"rm_tx_execution", "rm_tx_witness", "rm_outbox", "iam_authz_operations", "authz_permission_grants", "authz_assignments"} {
			var count int64
			require.NoError(t, db.Table(table).Count(&count).Error)
			result[table] = count
		}
		return result
	}
	before := counts()
	version, err := policyrepo.NewPolicyVersionRepository(db).GetCurrent(admin)
	require.NoError(t, err)
	for range 3 { // alternating role selections cannot reuse another role's data
		request(emptyID, "grants", http.StatusOK, 0)
		request(emptyID, "assignments", http.StatusOK, 0)
		request(populatedID, "grants", http.StatusOK, 1)
		request(populatedID, "assignments", http.StatusOK, 1)
	}
	actor = meta.ID(8)
	request(populatedID, "grants", http.StatusForbidden, 0)
	request(populatedID, "assignments", http.StatusForbidden, 0)
	actor = meta.ID(7) // can list ordinary roles, cannot see a protected role
	request(populatedID, "grants", http.StatusOK, 1)
	protected, err := rolerepo.NewRoleRepository(db).FindByName(admin, "platform_admin")
	require.NoError(t, err)
	request(protected.ID, "grants", http.StatusNotFound, 0)
	request(protected.ID, "assignments", http.StatusNotFound, 0)
	request(meta.ID(999999999), "grants", http.StatusNotFound, 0)
	require.Equal(t, before, counts(), "GET changed authorization or messaging facts")
	after, err := policyrepo.NewPolicyVersionRepository(db).GetCurrent(admin)
	require.NoError(t, err)
	require.Equal(t, version.Version, after.Version)
	require.Zero(t, sender.calls, "GET reached the transaction sender")
}
