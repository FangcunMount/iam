package scopemigrate

import (
	"context"
	"fmt"
	assignmentpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/assignment"
	grantpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/permissiongrant"
	policypo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/policy"
	resourcepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/resource"
	rolepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/role"
	dbmysql "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

type transactionalStager struct{ fail bool }

func (s transactionalStager) Stage(ctx context.Context, events ...event.DomainEvent) error {
	tx, err := dbmysql.RequireTx(ctx)
	if err != nil {
		return err
	}
	if err = tx.Exec("INSERT INTO test_outbox (id) VALUES(NULL)").Error; err != nil {
		return err
	}
	if s.fail {
		return fmt.Errorf("outbox failure")
	}
	return nil
}
func database(t *testing.T) *gorm.DB {
	t.Helper()
	if db := mysqlTestDatabase(t); db != nil {
		return db
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}
func applyFixture(t *testing.T) (*gorm.DB, *gorm.DB, Input) {
	t.Helper()
	iam, qs := database(t), database(t)
	if err := iam.AutoMigrate(&rolepo.RolePO{}, &assignmentpo.AssignmentPO{}, &grantpo.GrantPO{}, &resourcepo.ResourcePO{}, &policypo.PolicyVersionPO{}, &Receipt{}); err != nil {
		t.Fatal(err)
	}
	if iam.Dialector.Name() == "mysql" {
		if err := iam.Migrator().DropTable(&Receipt{}); err != nil {
			t.Fatal(err)
		}
		_, file, _, _ := runtime.Caller(0)
		ddl, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../pkg/migration/migrations/000037_scope_migration_receipts.up.sql"))
		if err != nil {
			t.Fatal(err)
		}
		if err = iam.Exec(string(ddl)).Error; err != nil {
			t.Fatal(err)
		}
	}
	outboxDDL := "CREATE TABLE test_outbox (id INTEGER PRIMARY KEY AUTOINCREMENT)"
	if iam.Dialector.Name() == "mysql" {
		outboxDDL = "CREATE TABLE test_outbox (id BIGINT PRIMARY KEY AUTO_INCREMENT)"
	}
	if err := iam.Exec(outboxDDL).Error; err != nil {
		t.Fatal(err)
	}
	state, _, input := fixture()
	state.Assignments[0].Version = 1
	state.Assignments[0].GrantedBy = "user:9"
	now := time.Now().UTC().Truncate(time.Microsecond)
	state.Assignments[0].CreatedAt = now
	state.Assignments[0].UpdatedAt = now
	state.Assignments[0].GrantedAt = now
	state.Roles[0].CreatedAt = now
	state.Roles[0].UpdatedAt = now
	policy := policypo.PolicyVersionPO{PolicyVersion: 5}
	policy.ID = 1
	policy.CreatedAt = now
	policy.UpdatedAt = now
	for _, v := range []any{&state.Roles[0], &state.Assignments[0], &policy} {
		if err := iam.Session(&gorm.Session{SkipHooks: true}).Create(v).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{"CREATE TABLE operators(id INTEGER,org_id INTEGER,user_id INTEGER,is_active BOOLEAN,deleted_at DATETIME)", "CREATE TABLE actor_stores(id INTEGER,org_id INTEGER,is_active BOOLEAN)", "INSERT INTO operators VALUES(4,7,3,TRUE,NULL)", "INSERT INTO actor_stores VALUES(8,7,TRUE)"} {
		if err := qs.Exec(query).Error; err != nil {
			t.Fatal(err)
		}
	}
	return iam, qs, input
}
func TestScopeApplyIdempotenceAndDrift(t *testing.T) {
	iam, qs, input := applyFixture(t)
	ctx := context.Background()
	p, err := Preflight(ctx, iam, qs, "scope-v1", input)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := Apply(ctx, iam, qs, transactionalStager{}, "scope-v1", "9", p.Fingerprint, input, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(ctx, iam, qs, "scope-v1"); err != nil {
		t.Fatal(err)
	}
	again, err := Apply(ctx, iam, qs, transactionalStager{}, "scope-v1", "9", p.Fingerprint, Input{}, true)
	if err != nil || again.AfterHash != receipt.AfterHash {
		t.Fatalf("replay %v", err)
	}
	var count int64
	iam.Table("test_outbox").Count(&count)
	if count != 1 {
		t.Fatalf("duplicate outbox %d", count)
	}
	if err := qs.Exec("UPDATE operators SET is_active=FALSE").Error; err != nil {
		t.Fatal(err)
	}
	report, err := Status(ctx, iam, qs, "scope-v1")
	if err != nil || report.State != "applied_drifted" {
		t.Fatalf("drift %+v %v", report, err)
	}
	if _, err := Apply(ctx, iam, qs, transactionalStager{}, "scope-v1", "9", p.Fingerprint, input, true); err == nil {
		t.Fatal("drift overwritten")
	}
	if _, err := Verify(ctx, iam, qs, "scope-v1"); err == nil {
		t.Fatal("drift passed verification")
	}
}
func TestScopeApplyOutboxFailureRollsBackEverything(t *testing.T) {
	iam, qs, input := applyFixture(t)
	ctx := context.Background()
	before, err := LoadSnapshot(ctx, iam, qs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(ctx, iam, qs, transactionalStager{fail: true}, "scope-v1", "9", before.Hash(), input, true); err == nil {
		t.Fatal("failed outbox committed")
	}
	after, err := LoadSnapshot(ctx, iam, qs)
	if err != nil || after.Hash() != before.Hash() {
		t.Fatalf("partial commit %v", err)
	}
	for _, table := range []string{"test_outbox", "iam_scope_migrations"} {
		var count int64
		iam.Table(table).Count(&count)
		if count != 0 {
			t.Fatalf("partial %s write", table)
		}
	}
}

func TestScopeApplyExplicitSecondCompanyCreatesOnlyReviewedAssignment(t *testing.T) {
	iam, qs, input := applyFixture(t)
	ctx := context.Background()
	if err := qs.Exec("INSERT INTO operators VALUES(5,9,3,TRUE,NULL)").Error; err != nil {
		t.Fatal(err)
	}
	input.Mappings[0].Targets = append(input.Mappings[0].Targets, Target{OrgID: "9", Kind: "all_stores", StoreIDs: []string{}})
	p, err := Preflight(ctx, iam, qs, "multi", input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(ctx, iam, qs, transactionalStager{}, "multi", "9", p.Fingerprint, input, true); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(ctx, iam, qs, "multi"); err != nil {
		t.Fatal(err)
	}
	var assignments []assignmentpo.AssignmentPO
	if err := iam.Order("org_id").Find(&assignments).Error; err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 2 || assignments[0].ID != 1 || assignments[1].ID == 1 || assignments[1].OrgID != 9 || assignments[1].GrantedBy != "user:9" {
		t.Fatalf("unexpected assignments %+v", assignments)
	}
}
