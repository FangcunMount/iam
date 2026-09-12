package operationscatalog

import (
	"context"
	"fmt"
	assignmentpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/assignment"
	grantpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/permissiongrant"
	policypo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/policy"
	resourcepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/resource"
	rolepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/role"
	dbctx "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"testing"
)

type outbox struct{ fail bool }

func (o outbox) Stage(ctx context.Context, _ ...event.DomainEvent) error {
	tx, e := dbctx.RequireTx(ctx)
	if e != nil {
		return e
	}
	if e = tx.Exec("INSERT INTO test_outbox VALUES (1)").Error; e != nil {
		return e
	}
	if o.fail {
		return fmt.Errorf("injected outbox failure")
	}
	return nil
}
func fixture(t *testing.T) *gorm.DB {
	db := mysqlDatabase(t)
	if db == nil {
		var e error
		db, e = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, e)
		pool, _ := db.DB()
		pool.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = pool.Close() })
	}
	require.NoError(t, db.AutoMigrate(&rolepo.RolePO{}, &resourcepo.ResourcePO{}, &grantpo.GrantPO{}, &assignmentpo.AssignmentPO{}, &policypo.PolicyVersionPO{}))
	for _, sql := range []string{"CREATE TABLE test_outbox (id INT)", "CREATE TABLE users (id BIGINT,status INT,deleted_at DATETIME)", "INSERT INTO users VALUES(9,1,NULL)", "INSERT INTO authz_roles(id,name,management_protection) VALUES(1,'platform_admin','protected'),(2,'qs:assessment_operator','standard'),(3,'qs:evaluation_plan_manager','standard'),(4,'qs:result_reviewer','standard')", "INSERT INTO authz_assignments(id,subject_type,subject_id,role_id,granted_by,granted_at) VALUES(1,'user',9,1,'test','2026-09-01')"} {
		require.NoError(t, db.Exec(sql).Error)
	}
	require.NoError(t, db.Create(&policypo.PolicyVersionPO{PolicyVersion: 5}).Error)
	return db
}
func TestInstallReplayAndConflicts(t *testing.T) {
	db := fixture(t)
	ctx := context.Background()
	before, e := Preflight(ctx, db)
	require.NoError(t, e)
	require.Len(t, before.MissingRoles, 3)
	after, e := Apply(ctx, db, outbox{}, before.Fingerprint, "9", true)
	require.NoError(t, e)
	require.Equal(t, "configured", after.State)
	require.EqualValues(t, 6, after.PolicyVersion)
	require.Len(t, after.GrantIDs, 3)
	repeated, e := Apply(ctx, db, outbox{}, after.Fingerprint, "9", true)
	require.NoError(t, e)
	require.Equal(t, after.Fingerprint, repeated.Fingerprint)
	var count int64
	require.NoError(t, db.Table("test_outbox").Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Table("authz_assignments").Count(&count).Error)
	require.EqualValues(t, 1, count)
	_, e = Apply(ctx, db, outbox{}, before.Fingerprint, "9", true)
	require.Error(t, e)
	require.NoError(t, db.Exec("UPDATE authz_resources SET actions='[\"read\",\"update\"]'").Error)
	_, e = Preflight(ctx, db)
	require.Error(t, e)
}
func TestOutboxFailureRollsBackCatalogAndPolicy(t *testing.T) {
	db := fixture(t)
	ctx := context.Background()
	before, e := Preflight(ctx, db)
	require.NoError(t, e)
	_, e = Apply(ctx, db, outbox{fail: true}, before.Fingerprint, "9", true)
	require.Error(t, e)
	after, e := Preflight(ctx, db)
	require.NoError(t, e)
	require.Equal(t, before.Fingerprint, after.Fingerprint)
	var count int64
	require.NoError(t, db.Table("test_outbox").Count(&count).Error)
	require.Zero(t, count)
}
