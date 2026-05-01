package casbinrule

import (
	"context"
	"testing"

	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRepositoryStoresPolicyRulesWithCasbinFieldOrder(t *testing.T) {
	t.Parallel()

	db := setupCasbinRuleDB(t)
	repo := NewRepository(db)

	permission, err := authzDomain.NewPermission("iam:admin", "tenant-a", "iam:user:*", "read")
	require.NoError(t, err)
	require.NoError(t, repo.AddPermission(context.Background(), permission))

	var rows []rulePO
	require.NoError(t, db.Order("id").Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, "p", rows[0].PType)
	require.Equal(t, "role:iam:admin", valueOf(rows[0].V0))
	require.Equal(t, "tenant-a", valueOf(rows[0].V1))
	require.Equal(t, "iam:user:*", valueOf(rows[0].V2))
	require.Equal(t, "read", valueOf(rows[0].V3))
	require.Equal(t, "all:*", valueOf(rows[0].V4))

	require.NoError(t, repo.RemovePermission(context.Background(), permission))
	rows = nil
	require.NoError(t, db.Find(&rows).Error)
	require.Empty(t, rows)
}

func TestRepositoryStoresScopedPolicyRulesWithCasbinFieldOrder(t *testing.T) {
	t.Parallel()

	db := setupCasbinRuleDB(t)
	repo := NewRepository(db)
	scope, err := authzDomain.NewScope(authzDomain.ScopeKindOrigin, "1")
	require.NoError(t, err)
	permission, err := authzDomain.NewPermission(
		"iam:admin",
		"tenant-a",
		"iam:user:*",
		"update",
		authzDomain.WithPermissionScope(scope),
	)
	require.NoError(t, err)
	require.NoError(t, repo.AddPermission(context.Background(), permission))

	var rows []rulePO
	require.NoError(t, db.Order("id").Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, "origin:1", valueOf(rows[0].V4))

	require.NoError(t, repo.RemovePermission(context.Background(), permission))
	rows = nil
	require.NoError(t, db.Find(&rows).Error)
	require.Empty(t, rows)
}

func TestRepositoryRemovesLegacyEmptyScopePolicyAsDefaultAllScope(t *testing.T) {
	t.Parallel()

	db := setupCasbinRuleDB(t)
	repo := NewRepository(db)
	require.NoError(t, db.Create(&rulePO{
		PType: "p",
		V0:    stringPtr("role:iam:admin"),
		V1:    stringPtr("tenant-a"),
		V2:    stringPtr("iam:user:*"),
		V3:    stringPtr("read"),
		V4:    stringPtr(""),
	}).Error)

	permission, err := authzDomain.NewPermission("iam:admin", "tenant-a", "iam:user:*", "read")
	require.NoError(t, err)
	require.NoError(t, repo.RemovePermission(context.Background(), permission))

	var count int64
	require.NoError(t, db.Model(&rulePO{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestRepositoryStoresGroupingRulesWithCasbinFieldOrder(t *testing.T) {
	t.Parallel()

	db := setupCasbinRuleDB(t)
	repo := NewRepository(db)

	subject, err := authzDomain.NewSubject(authzDomain.SubjectTypeUser, "100")
	require.NoError(t, err)
	binding, err := authzDomain.NewRoleBinding(subject, "iam:admin", "tenant-a", "operator-1")
	require.NoError(t, err)
	require.NoError(t, repo.AddRoleBinding(context.Background(), binding))

	var rows []rulePO
	require.NoError(t, db.Order("id").Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, "g", rows[0].PType)
	require.Equal(t, "user:100", valueOf(rows[0].V0))
	require.Equal(t, "role:iam:admin", valueOf(rows[0].V1))
	require.Equal(t, "tenant-a", valueOf(rows[0].V2))

	require.NoError(t, repo.RemoveRoleBinding(context.Background(), binding))
	rows = nil
	require.NoError(t, db.Find(&rows).Error)
	require.Empty(t, rows)
}

func setupCasbinRuleDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&rulePO{}))
	return db
}

func valueOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
