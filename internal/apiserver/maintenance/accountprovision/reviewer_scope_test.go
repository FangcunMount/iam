package accountprovision

import (
	"context"
	"fmt"
	"testing"
	"time"

	crypto "github.com/FangcunMount/iam/v5/internal/apiserver/infra/crypto"
	assignmentpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/assignment"
	credentialpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/credential"
	loginpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/loginidentity"
	grantpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/permissiongrant"
	policypo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/policy"
	resourcepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/resource"
	rolepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/role"
	userpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/user"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/rolemodel"
	database "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type scopeTestStager struct{ fail bool }

func (s scopeTestStager) Stage(ctx context.Context, events ...event.DomainEvent) error {
	tx, e := database.RequireTx(ctx)
	if e != nil {
		return e
	}
	if e = tx.Exec("INSERT INTO test_scope_outbox(value) VALUES(1)").Error; e != nil {
		return e
	}
	if s.fail {
		return fmt.Errorf("injected outbox failure")
	}
	return nil
}
func reviewerFixture(t *testing.T) (*gorm.DB, Input) {
	t.Helper()
	db := testDatabase(t)
	if db == nil {
		var err error
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, err)
		pool, err := db.DB()
		require.NoError(t, err)
		pool.SetMaxOpenConns(1)
		t.Cleanup(func() { require.NoError(t, pool.Close()) })
	}
	require.NoError(t, db.AutoMigrate(&userpo.UserPO{}, &loginpo.PO{}, &credentialpo.V2PO{}, &rolepo.RolePO{}, &assignmentpo.AssignmentPO{}, &grantpo.GrantPO{}, &resourcepo.ResourcePO{}, &policypo.PolicyVersionPO{}))
	require.NoError(t, db.Exec("CREATE TABLE test_scope_outbox(value INTEGER)").Error)
	now := time.Now().UTC().Truncate(time.Second)
	user := userpo.UserPO{}
	user.ID = 10001
	user.Name = "system"
	user.Status = 1
	user.CreatedAt = now
	user.UpdatedAt = now
	policy := policypo.PolicyVersionPO{PolicyVersion: 158}
	policy.ID = 1
	policy.CreatedAt = now
	policy.UpdatedAt = now
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&policy).Error)
	for i, name := range []string{"platform_admin", "iam_admin", "qs:admin"} {
		role := rolepo.RolePO{Name: name, ManagementProtection: "protected"}
		role.ID = meta.ID(i + 1)
		role.CreatedAt = now
		role.UpdatedAt = now
		require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&role).Error)
		a := assignmentpo.AssignmentPO{SubjectType: "user", SubjectID: "10001", RoleID: uint64(i + 1), GrantedAt: now, GrantedBy: "user:10001"}
		a.ID = meta.ID(i + 1)
		a.Version = 1
		a.CreatedAt = now
		a.UpdatedAt = now
		if name != "iam_admin" {
			raw := "[]"
			a.OrgID = 1
			a.ScopeKind = "all_stores"
			a.ScopeStoreIDs = &raw
		}
		require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&a).Error)
	}
	input := Input{RequestID: "reviewer-10002-actions", ActorID: "10001", UserID: "10002", Username: "review@mfangcunmount.com", Name: "安全与产品审核员", Reason: "explicit fixture", Password: "PrivateTestPassword-123456"}
	_, err := Apply(context.Background(), db, input, crypto.NewArgon2Hasher(""), input.fingerprint())
	require.NoError(t, err)
	for i := 1; i <= 3; i++ {
		a := assignmentpo.AssignmentPO{SubjectType: "user", SubjectID: "10002", RoleID: uint64(i), GrantedAt: now, GrantedBy: "user:10001"}
		a.ID = meta.ID(i + 10)
		a.Version = 1
		a.CreatedAt = now
		a.UpdatedAt = now
		require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&a).Error)
	}
	return db, input
}
func TestReviewerScopeRepairPreservesOtherFactsAndOutboxAtomicity(t *testing.T) {
	db, input := reviewerFixture(t)
	ctx := context.Background()
	hasher := crypto.NewArgon2Hasher("")
	if db.Dialector.Name() == "mysql" {
		pool, e := db.DB()
		require.NoError(t, e)
		pool.SetMaxOpenConns(1)
		require.NoError(t, db.Exec("SET time_zone = '+08:00'").Error)
		require.NoError(t, db.Exec("UPDATE authz_assignments SET updated_at=CURRENT_TIMESTAMP").Error)
	}
	p, err := ReviewerScopePreflight(ctx, db, input, hasher, 1)
	require.NoError(t, err)
	require.Len(t, p.Plan.Changes, 2)
	_, err = ReviewerScopeApply(ctx, db, input, hasher, 1, p.Fingerprint, scopeTestStager{fail: true})
	require.Error(t, err)
	current, err := ReviewerScopePreflight(ctx, db, input, hasher, 1)
	require.NoError(t, err)
	require.Equal(t, p.Fingerprint, current.Fingerprint)
	var count int64
	require.NoError(t, db.Table("test_scope_outbox").Count(&count).Error)
	require.Zero(t, count)
	fixed, err := ReviewerScopeApply(ctx, db, input, hasher, 1, p.Fingerprint, scopeTestStager{})
	require.NoError(t, err)
	require.Equal(t, "scope_matches_reference", fixed.State)
	require.Equal(t, int64(159), fixed.After.PolicyVersion)
	current, err = ReviewerScopePreflight(ctx, db, input, hasher, 1)
	require.NoError(t, err)
	require.Empty(t, current.Plan.Changes)
	_, err = ReviewerScopeApply(ctx, db, input, hasher, 1, p.Fingerprint, scopeTestStager{})
	require.Error(t, err, "stale plan cannot be replayed")
	_, err = ReviewerScopeApply(ctx, db, input, hasher, 1, current.Fingerprint, scopeTestStager{})
	require.NoError(t, err)
	require.NoError(t, db.Table("test_scope_outbox").Count(&count).Error)
	require.Equal(t, int64(1), count)
}
func TestReviewerScopeRejectsSourceMissingScopeTargetConflictAndDrift(t *testing.T) {
	db, input := reviewerFixture(t)
	ctx := context.Background()
	hasher := crypto.NewArgon2Hasher("")
	p, err := ReviewerScopePreflight(ctx, db, input, hasher, 1)
	require.NoError(t, err)
	_, err = ReviewerScopePreflight(ctx, db, input, hasher, 2)
	require.Error(t, err)
	require.NoError(t, db.Exec("UPDATE authz_policy_versions SET policy_version=159 WHERE id=1").Error)
	_, err = ReviewerScopeApply(ctx, db, input, hasher, 1, p.Fingerprint, scopeTestStager{})
	require.Error(t, err)
	state, err := rolemodel.LoadState(ctx, db)
	require.NoError(t, err)
	state.Assignments[0].ScopeStoreIDs = nil
	_, err = reviewerScopePlan(state, 1)
	require.Error(t, err)
	state, err = rolemodel.LoadState(ctx, db)
	require.NoError(t, err)
	state.Assignments[3].OrgID = 2
	_, err = reviewerScopePlan(state, 1)
	require.Error(t, err)
	input.UserID = "10003"
	_, err = ReviewerScopePreflight(ctx, db, input, hasher, 1)
	require.Error(t, err)
}
