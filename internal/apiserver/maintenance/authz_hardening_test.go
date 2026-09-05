package maintenance

import (
	"context"
	"fmt"
	"testing"

	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/objectattributeadmission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	assignmentrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/assignment"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/eventoutbox"
	grantrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/permissiongrant"
	resourcerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/resource"
	rolerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	authzuow "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v3/internal/apiserver/testfixtures/authzdb"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/FangcunMount/iam/v3/pkg/event"
	"github.com/FangcunMount/iam/v3/pkg/eventcatalog"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedHardening(t *testing.T, db *gorm.DB) (*objectattributeadmission.Registry, event.Stager) {
	t.Helper()
	ctx := context.Background()
	roles := rolerepo.NewRoleRepository(db)
	resources := resourcerepo.NewResourceRepository(db)
	grants := grantrepo.NewRepository(db)
	catalog, err := resource.NewResource("iam:authz:collection:resources", []string{"create", "update", "delete", "read", "list", "validate_action"}, resource.WithDisplayName("Catalog"))
	require.NoError(t, err)
	require.NoError(t, resources.Create(ctx, &catalog))
	require.NoError(t, db.Exec("INSERT INTO users (id,status) VALUES (1,1)").Error)
	for _, tenant := range []string{"platform", "a", "b"} {
		r, err := role.NewRole("operator", "Operator", tenant)
		require.NoError(t, err)
		require.NoError(t, roles.Create(ctx, &r))
		grant, err := permissiongrant.NewSystem(r.ID, tenant, resource.ResourceID{}, catalog.KeyString(), "*", constraint.Empty(), "seed")
		require.NoError(t, err)
		require.NoError(t, grants.Create(ctx, &grant))
		if tenant == "platform" {
			a, err := assignment.NewAssignment(assignment.SubjectTypeUser, meta.ID(1), r.ID, tenant, assignment.WithGrantedBy("seed"))
			require.NoError(t, err)
			require.NoError(t, assignmentrepo.NewRepository(db).Create(ctx, &a))
		} else {
			broad, err := permissiongrant.NewSystem(r.ID, tenant, resource.ResourceID{}, "iam:authz:collection:*", "*", constraint.Empty(), "seed")
			require.NoError(t, err)
			require.NoError(t, grants.Create(ctx, &broad))
			write, err := permissiongrant.New(r.ID, tenant, catalog.ID, catalog.KeyString(), "create", constraint.Empty(), "seed")
			require.NoError(t, err)
			require.NoError(t, grants.Create(ctx, &write))
			read, err := permissiongrant.New(r.ID, tenant, catalog.ID, catalog.KeyString(), "read", constraint.Empty(), "seed")
			require.NoError(t, err)
			require.NoError(t, grants.Create(ctx, &read))
		}
	}
	policy, err := objectattributeadmission.New(nil)
	require.NoError(t, err)
	cfg, err := eventcatalog.Parse([]byte(`version: "1"
topics:
  version:
    name: iam.authz.version
events:
  iam.authz.version_changed:
    topic: version
    delivery: durable_outbox
    aggregate: PolicyVersion
    domain: authz
    handler: iam-policy-sync
`))
	require.NoError(t, err)
	return policy, eventoutbox.NewStore(db, eventcatalog.NewCatalog(cfg))
}
func TestAuthzHardeningTransactionContracts(t *testing.T) {
	for _, realMySQL := range []bool{false, true} {
		t.Run(fmt.Sprintf("mysql=%v", realMySQL), func(t *testing.T) {
			db := authzdb.Open(t, realMySQL)
			providers, stager := seedHardening(t, db)
			ctx := context.Background()
			before, err := AnalyzeAuthzHardening(ctx, db, providers)
			require.NoError(t, err)
			require.Empty(t, before.Blockers)
			require.Len(t, before.Changes, 4)
			require.Equal(t, []string{"user:1"}, before.PlatformWriters)
			again, err := AnalyzeAuthzHardening(ctx, db, providers)
			require.NoError(t, err)
			require.Equal(t, before.Fingerprint, again.Fingerprint)
			_, err = ApplyAuthzHardening(ctx, db, authzuow.NewUnitOfWork(db, nil, stager), providers, fmt.Sprintf("%064d", 0))
			require.Error(t, err)
			_, err = ApplyAuthzHardening(ctx, db, authzuow.NewUnitOfWork(db, nil, failingHardeningStager{}), providers, before.Fingerprint)
			require.Error(t, err)
			afterFailure, err := AnalyzeAuthzHardening(ctx, db, providers)
			require.NoError(t, err)
			require.Equal(t, before.Fingerprint, afterFailure.Fingerprint)
			result, err := ApplyAuthzHardening(ctx, db, authzuow.NewUnitOfWork(db, nil, stager), providers, before.Fingerprint)
			require.NoError(t, err)
			require.True(t, result.Complete)
			require.Len(t, result.AppliedChanges, 4)
			require.Equal(t, map[string]int64{"a": 1, "b": 1}, result.PolicyVersions)
			var events, history, broad int64
			require.NoError(t, db.Model(&eventoutbox.OutboxPO{}).Count(&events).Error)
			require.EqualValues(t, 2, events)
			require.NoError(t, db.Model(&grantrepo.GrantPO{}).Where("revoked_at IS NOT NULL").Count(&history).Error)
			require.EqualValues(t, 4, history)
			require.NoError(t, db.Model(&grantrepo.GrantPO{}).Where("resource_pattern = ? AND revoked_at IS NULL", "iam:authz:collection:*").Count(&broad).Error)
			require.EqualValues(t, 2, broad)
			idempotent, err := ApplyAuthzHardening(ctx, db, authzuow.NewUnitOfWork(db, nil, stager), providers, result.Fingerprint)
			require.NoError(t, err)
			require.Equal(t, result.Fingerprint, idempotent.Fingerprint)
			require.NoError(t, db.Model(&eventoutbox.OutboxPO{}).Count(&events).Error)
			require.EqualValues(t, 2, events)
		})
	}
}

type failingHardeningStager struct{}

func (failingHardeningStager) Stage(context.Context, ...event.DomainEvent) error {
	return fmt.Errorf("injected outbox failure")
}
func TestHardeningPreflightReportsConcreteInvalidReferences(t *testing.T) {
	db := authzdb.Open(t, false)
	providers, _ := seedHardening(t, db)
	require.NoError(t, db.Model(&assignmentrepo.AssignmentPO{}).Where("tenant_id = ?", "platform").Update("role_id", 999).Error)
	report, err := AnalyzeAuthzHardening(context.Background(), db, providers)
	require.NoError(t, err)
	require.Contains(t, report.Blockers[0].Detail, "999")
	require.NotEmpty(t, report.Blockers[0].Object)
}
