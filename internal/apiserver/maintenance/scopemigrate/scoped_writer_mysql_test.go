package scopemigrate

import (
	"context"
	cbErrors "github.com/FangcunMount/component-base/pkg/errors"
	assignmentApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/assignment"
	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/management"
	assignment "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/assignment"
	policy "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	role "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/authz/subjectresolver"
	assignmentRepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/assignment"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	policyRepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/policy"
	roleRepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/role"
	authzUOW "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

type scopedWriterUserResolver struct{}

func (scopedWriterUserResolver) ResolveUser(context.Context, meta.ID) error { return nil }

func TestConcurrentScopedAssignmentWritersRejectStaleVersionMySQL(t *testing.T) {
	if os.Getenv("SCOPE_MIGRATION_MYSQL_DSN") == "" {
		if os.Getenv("SCOPE_MIGRATION_REQUIRE_MYSQL") == "true" {
			t.Fatal("SCOPE_MIGRATION_MYSQL_DSN required")
		}
		t.Skip("isolated MySQL required")
	}
	db := mysqlTestDatabase(t)
	require.NoError(t, db.AutoMigrate(&roleRepo.RolePO{}, &assignmentRepo.AssignmentPO{}, &policyRepo.PolicyVersionPO{}, &eventoutbox.OutboxPO{}))
	ctx, cancel := context.WithTimeout(management.WithAuthenticatedService(context.Background(), "admin"), 15*time.Second)
	defer cancel()
	roles := roleRepo.NewRoleRepository(db)
	r, err := role.NewRole("qs:assessment_operator", "operator")
	require.NoError(t, err)
	require.NoError(t, roles.Create(ctx, &r))
	version := policyRepo.PolicyVersionPO{PolicyVersion: 5}
	version.ID = 1
	require.NoError(t, db.Create(&version).Error)
	config, err := eventcatalog.Load("../../../../configs/events.yaml")
	require.NoError(t, err)
	stager := eventoutbox.NewStore(db, eventcatalog.NewCatalog(config))
	resolver := subjectresolver.NewUserSubjectResolver(scopedWriterUserResolver{})
	service := assignmentApp.NewCommandService(assignment.NewValidator(roles, resolver), roles, authzUOW.NewUnitOfWork(db, resolver, stager), nil, management.NewGuard(nil))
	sub, err := subject.NewUserRef(100)
	require.NoError(t, err)
	blocker := db.WithContext(ctx).Begin()
	require.NoError(t, blocker.Error)
	defer blocker.Rollback()
	var id uint64
	require.NoError(t, blocker.Raw("SELECT id FROM authz_roles WHERE id=? FOR UPDATE", r.ID).Scan(&id).Error)
	var schema string
	require.NoError(t, db.Raw("SELECT DATABASE()").Scan(&schema).Error)
	type outcome struct {
		store meta.ID
		err   error
	}
	done := make(chan outcome, 2)
	for _, storeID := range []meta.ID{10, 20} {
		desired, err := scope.New(1, scope.Stores, []meta.ID{storeID})
		require.NoError(t, err)
		cmd := assignmentApp.ReplaceManagedAssignmentsCommand{Subject: sub, OrgID: 1, ExpectedPolicyVersion: 5, ManagedRoleNames: []string{r.Name.String()}, ScopedRoles: []assignment.ScopedRoleGrant{{RoleName: r.Name, Scope: desired}}, ChangedBy: "user:200"}
		go func(storeID meta.ID) {
			_, err := service.ReplaceManagedAssignments(ctx, cmd)
			done <- outcome{storeID, err}
		}(storeID)
	}
	require.Eventually(t, func() bool {
		var count int64
		err := db.WithContext(ctx).Raw(`SELECT COUNT(DISTINCT w.REQUESTING_ENGINE_TRANSACTION_ID) FROM performance_schema.data_lock_waits w JOIN performance_schema.data_locks l ON l.ENGINE_LOCK_ID=w.REQUESTING_ENGINE_LOCK_ID AND l.ENGINE=w.ENGINE WHERE l.OBJECT_SCHEMA=? AND l.OBJECT_NAME='authz_roles'`, schema).Scan(&count).Error
		return err == nil && count >= 2
	}, 5*time.Second, 20*time.Millisecond)
	require.NoError(t, blocker.Commit().Error)
	var winner meta.ID
	for i := 0; i < 2; i++ {
		select {
		case result := <-done:
			if result.err == nil {
				require.Zero(t, winner, "both stale writers succeeded")
				winner = result.store
			} else {
				require.True(t, cbErrors.Is(result.err, policy.ErrStaleVersion), "unexpected write failure: %v", result.err)
			}
		case <-ctx.Done():
			t.Fatal("scoped writers did not complete")
		}
	}
	require.NotZero(t, winner)
	current, err := policyRepo.NewPolicyVersionRepository(db).GetCurrent(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 6, current.Version)
	rows, err := assignmentRepo.NewRepository(db).ListBySubject(ctx, assignment.SubjectTypeUser, 100)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	persisted, ok := rows[0].Scope()
	require.True(t, ok)
	require.True(t, persisted.ContainsStore(1, winner))
	var events int64
	require.NoError(t, db.Model(&eventoutbox.OutboxPO{}).Count(&events).Error)
	require.EqualValues(t, 1, events, "stale writer must not publish another policy event")
}
