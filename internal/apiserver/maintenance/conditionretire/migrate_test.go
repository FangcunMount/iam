package conditionretire

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"

	legacygrant "github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/legacycondition/grant"

	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/resource"
	ap "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/assignment"
	gp "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/permissiongrant"
	pp "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/policy"
	rp "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/resource"
	roles "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/role"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/legacycondition/constraint"
	gd "github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/legacycondition/grant"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/rolemodel"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type stager struct {
	fail  bool
	calls int
}

func (s *stager) Stage(_ context.Context, _ ...event.DomainEvent) error {
	if s.fail {
		return fmt.Errorf("outbox failed")
	}
	s.calls++
	return nil
}
func fixture(t *testing.T, existing bool) *gorm.DB {
	t.Helper()
	db := testDatabase(t)
	require.NoError(t, db.AutoMigrate(&roles.RolePO{}, &ap.AssignmentPO{}, &gp.GrantPO{}, &rp.ResourcePO{}, &pp.PolicyVersionPO{}, &Receipt{}))
	r := roles.RolePO{Name: rolemodel.Operator, ManagementProtection: "standard"}
	require.NoError(t, db.Create(&r).Error)
	res := rp.ResourcePO{Key: assessment, Actions: `["retry"]`, AttributeSchema: `{"version":1,"attributes":[{"key":"object.origin_type","type":"string","allowed_string_values":["adhoc","plan"]}]}`}
	require.NoError(t, db.Create(&res).Error)
	c, err := constraint.New(constraint.Equal("object.origin_type", constraint.StringValue("adhoc")))
	require.NoError(t, err)
	for _, cond := range []constraint.Set{c, constraint.Empty()} {
		if cond.IsUnconditional() && !existing {
			continue
		}
		g, err := gd.New(r.ID, resource.NewResourceID(res.ID.Uint64()), assessment, "retry", cond, "seed")
		require.NoError(t, err)
		p, err := (legacygrant.Mapper{}).ToPO(&g)
		require.NoError(t, err)
		require.NoError(t, db.Create(p).Error)
	}
	require.NoError(t, db.Create(&ap.AssignmentPO{SubjectType: "user", SubjectID: "123", RoleID: r.ID.Uint64(), GrantedBy: "seed"}).Error)
	require.NoError(t, db.Create(&pp.PolicyVersionPO{PolicyVersion: 12}).Error)
	return db
}
func TestRetirementLifecycle(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			db := fixture(t, existing)
			ctx := context.Background()
			s := &stager{}
			p, err := Preflight(ctx, db)
			require.NoError(t, err)
			require.Len(t, p.Plan.Grants, 1)
			require.Len(t, p.Plan.Users, 1)
			if existing {
				require.Empty(t, p.Plan.Users[0].AddedOrigins)
			} else {
				require.Equal(t, []string{"plan"}, p.Plan.Users[0].AddedOrigins)
			}
			_, err = Apply(ctx, db, s, p.Fingerprint, true)
			require.NoError(t, err)
			r, err := Verify(ctx, db)
			require.NoError(t, err)
			require.Equal(t, "applied_unchanged", r.State)
			before, err := rolemodel.LoadState(ctx, db)
			require.NoError(t, err)
			_, err = Preflight(ctx, db)
			require.NoError(t, err)
			_, err = Apply(ctx, db, s, p.Fingerprint, true)
			require.NoError(t, err)
			after, err := rolemodel.LoadState(ctx, db)
			require.NoError(t, err)
			require.Equal(t, before.Hash(), after.Hash())
			require.Equal(t, 1, s.calls)
			_, err = Rollback(ctx, db, s, p.Fingerprint, true)
			require.NoError(t, err)
			r, err = Status(ctx, db)
			require.NoError(t, err)
			require.Equal(t, "rolled_back", r.State)
			restored, err := rolemodel.LoadState(ctx, db)
			require.NoError(t, err)
			require.Equal(t, int64(14), restored.PolicyVersion)
			require.Len(t, Build(restored).Grants, 1)
			if existing {
				require.NotEmpty(t, Build(restored).Grants[0].ExistingID)
			}
			_, err = Apply(ctx, db, s, p.Fingerprint, true)
			require.Error(t, err)
		})
	}
}
func TestFailureDriftAndCorruption(t *testing.T) {
	db := fixture(t, false)
	ctx := context.Background()
	p, err := Preflight(ctx, db)
	require.NoError(t, err)
	before, _ := rolemodel.LoadState(ctx, db)
	_, err = Apply(ctx, db, &stager{fail: true}, p.Fingerprint, true)
	require.Error(t, err)
	after, _ := rolemodel.LoadState(ctx, db)
	require.Equal(t, before.Hash(), after.Hash())
	_, err = Apply(ctx, db, &stager{}, p.Fingerprint, true)
	require.NoError(t, err)
	require.NoError(t, db.Table("authz_roles").Where("name = ?", rolemodel.Operator).Update("description", "later change").Error)
	r, err := Status(ctx, db)
	require.NoError(t, err)
	require.Equal(t, "applied_drifted", r.State)
	require.Contains(t, r.Differences, "roles")
	_, err = Verify(ctx, db)
	require.Error(t, err)
	_, err = Apply(ctx, db, &stager{}, p.Fingerprint, true)
	require.Error(t, err)
	_, err = Rollback(ctx, db, &stager{}, p.Fingerprint, true)
	require.Error(t, err)
	require.NoError(t, db.Model(&Receipt{}).Where("migration_id = ?", MigrationID).Update("before_json", "{}").Error)
	r, err = Status(ctx, db)
	require.Error(t, err)
	require.Equal(t, "invalid", r.State)
}
func TestPreflightRejectsUnknownConditions(t *testing.T) {
	db := fixture(t, false)
	require.NoError(t, db.Table("authz_permission_grants").Where("action = ?", "retry").Update("constraint_set", `{"version":1,"all_of":[],"unknown":true}`).Error)
	r, err := Preflight(context.Background(), db)
	require.Error(t, err)
	require.NotEmpty(t, r.Plan.Issues)
}

func TestDurableOutboxAndPolicyVersionAreIdempotent(t *testing.T) {
	db := fixture(t, false)
	ctx := context.Background()
	require.NoError(t, db.AutoMigrate(&eventoutbox.OutboxPO{}))
	cfg, err := eventcatalog.Load("../../../../configs/events.yaml")
	require.NoError(t, err)
	store, err := eventoutbox.NewBootstrapStager(eventcatalog.NewCatalog(cfg))
	if err != nil {
		t.Fatal(err)
	}
	p, err := Preflight(ctx, db)
	require.NoError(t, err)
	_, err = Apply(ctx, db, store, p.Fingerprint, true)
	require.NoError(t, err)
	_, err = Apply(ctx, db, store, p.Fingerprint, true)
	require.NoError(t, err)
	var count int64
	require.NoError(t, db.Model(&eventoutbox.OutboxPO{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	_, err = Rollback(ctx, db, store, p.Fingerprint, true)
	require.NoError(t, err)
	require.NoError(t, db.Model(&eventoutbox.OutboxPO{}).Count(&count).Error)
	require.EqualValues(t, 2, count)
}
func TestChangedFactsAndIncorrectFingerprintRejectBeforeWrites(t *testing.T) {
	db := fixture(t, false)
	ctx := context.Background()
	p, err := Preflight(ctx, db)
	require.NoError(t, err)
	_, err = Apply(ctx, db, &stager{}, strings.Repeat("0", 64), true)
	require.Error(t, err)
	require.NoError(t, db.Table("authz_roles").Where("name = ?", rolemodel.Operator).Update("description", "concurrent write").Error)
	before, _ := rolemodel.LoadState(ctx, db)
	_, err = Apply(ctx, db, &stager{}, p.Fingerprint, true)
	require.Error(t, err)
	after, _ := rolemodel.LoadState(ctx, db)
	require.Equal(t, before.Hash(), after.Hash())
}
