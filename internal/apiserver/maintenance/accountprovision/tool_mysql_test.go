package accountprovision

import (
	"context"
	"database/sql"
	"fmt"
	crypto "github.com/FangcunMount/iam/v5/internal/apiserver/infra/crypto"
	credentialrepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/credential"
	loginrepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/loginidentity"
	userrepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/user"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"strings"
	"testing"
	"time"
)

func testDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	raw := os.Getenv("ACCOUNT_PROVISION_MYSQL_DSN")
	if raw == "" {
		if os.Getenv("ACCOUNT_PROVISION_REQUIRE_MYSQL") == "true" {
			t.Fatal("ACCOUNT_PROVISION_MYSQL_DSN required")
		}
		return nil
	}
	cfg, err := driver.ParseDSN(raw)
	if err != nil {
		t.Fatal("invalid isolated MySQL DSN")
	}
	if cfg.DBName != "" {
		t.Fatal("test DSN must not select an existing database")
	}
	cfg.ParseTime = true
	cfg.MultiStatements = true
	cfg.Loc = time.UTC
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("cannot open isolated MySQL admin")
	}
	name := fmt.Sprintf("iam_account_test_%d", time.Now().UnixNano())
	if !strings.HasPrefix(name, "iam_account_test_") {
		t.Fatal("unsafe test database")
	}
	if _, err := admin.Exec("CREATE DATABASE `" + name + "`"); err != nil {
		if closeErr := admin.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal(err)
	}
	cfg.DBName = name
	db, err := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		if _, e := admin.Exec("DROP DATABASE `" + name + "`"); e != nil {
			t.Error(e)
		}
		if closeErr := admin.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal("cannot connect to isolated MySQL test database")
	}
	t.Cleanup(func() {
		pool, _ := db.DB()
		if closeErr := pool.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		if _, e := admin.Exec("DROP DATABASE `" + name + "`"); e != nil {
			t.Error(e)
		}
		if closeErr := admin.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	return db
}

func TestMySQLProvisionUsesSignupTransactionWithoutGrantsAndNeverResetsExistingPasswords(t *testing.T) {
	db := testDatabase(t)
	if db == nil {
		t.Skip("set ACCOUNT_PROVISION_MYSQL_DSN")
	}
	if err := db.AutoMigrate(&userrepo.UserPO{}, &loginrepo.PO{}, &credentialrepo.V2PO{}); err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{
		"CREATE TABLE authz_roles(id BIGINT PRIMARY KEY,name VARCHAR(64),management_protection VARCHAR(32),deleted_at DATETIME)",
		"CREATE TABLE authz_assignments(id BIGINT PRIMARY KEY,subject_type VARCHAR(16),subject_id VARCHAR(64),role_id BIGINT,deleted_at DATETIME)",
		"CREATE TABLE authz_policy_versions(id BIGINT PRIMARY KEY,policy_version BIGINT)",
		"INSERT INTO authz_policy_versions VALUES(1,136)",
		"INSERT INTO users(id,name,email,status,created_at,updated_at,created_by,updated_by,deleted_by,version) VALUES(10001,'system','',1,NOW(),NOW(),0,0,0,1)",
		"INSERT INTO authz_roles VALUES(1,'platform_admin','protected',NULL)",
		"INSERT INTO authz_assignments VALUES(1,'user','10001',1,NULL)",
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	input := Input{RequestID: "account-test-1", ActorID: "10001", Username: "store_test_admin", Name: "专用门店管理员", Reason: "explicit provisioning test", Password: "PrivateTestPassword-123456"}
	hasher := crypto.NewArgon2Hasher("test-pepper")
	before, err := Preflight(context.Background(), db, input, hasher)
	if err != nil || before.State != "pending" {
		t.Fatal(before, err)
	}
	if _, err = Apply(context.Background(), db, input, hasher, "wrong"); err == nil {
		t.Fatal("wrong fingerprint accepted")
	}
	// A failure after user and identity inserts must roll back all three signup facts.
	if err = db.Exec("CREATE TRIGGER refuse_password BEFORE INSERT ON auth_credentials FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected credential failure'").Error; err != nil {
		t.Fatal(err)
	}
	if _, err = Apply(context.Background(), db, input, hasher, before.Fingerprint); err == nil {
		t.Fatal("credential failure ignored")
	}
	for _, table := range []string{"users", "auth_login_identities", "auth_credentials"} {
		var count int64
		if err = db.Table(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		want := int64(0)
		if table == "users" {
			want = 1
		}
		if count != want {
			t.Fatal("partial signup persisted", table, count)
		}
	}
	if err = db.Exec("DROP TRIGGER refuse_password").Error; err != nil {
		t.Fatal(err)
	}
	done, err := Apply(context.Background(), db, input, hasher, before.Fingerprint)
	if err != nil || done.State != "historical_completed" || done.UserID == "" {
		t.Fatal(done, err)
	}
	again, err := Apply(context.Background(), db, input, hasher, before.Fingerprint)
	if err != nil || again.UserID != done.UserID {
		t.Fatal("replay not idempotent", again, err)
	}
	var count int64
	if err = db.Table("authz_assignments").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("signup granted roles", count, err)
	}
	var policy int64
	if err = db.Table("authz_policy_versions").Select("policy_version").Scan(&policy).Error; err != nil || policy != 136 {
		t.Fatal("signup changed policy", policy, err)
	}
	var creator string
	if err = db.Table("users").Select("created_by").Where("id=?", done.UserID).Scan(&creator).Error; err != nil || creator != "10001" {
		t.Fatal("missing actor audit", creator, err)
	}
	changed := input
	changed.Password = "DifferentPrivatePassword-456789"
	if _, err = Apply(context.Background(), db, changed, hasher, before.Fingerprint); err == nil {
		t.Fatal("existing password silently reset or adopted")
	}
	collision := input
	collision.RequestID = "different-request"
	if _, err = Preflight(context.Background(), db, collision, hasher); err == nil {
		t.Fatal("existing login adopted by another request")
	}

	// Competing requests for a username may not adopt each other's new identity.
	type outcome struct {
		report Report
		err    error
	}
	results := make(chan outcome, 2)
	for _, id := range []string{"race-one", "race-two"} {
		candidate := input
		candidate.Username = "store_race_admin"
		candidate.RequestID = id
		go func(value Input) {
			report, err := Apply(context.Background(), db, value, hasher, value.fingerprint())
			results <- outcome{report, err}
		}(candidate)
	}
	successes := 0
	for i := 0; i < 2; i++ {
		got := <-results
		if got.err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatal("concurrent requests adopted the same identity", successes)
	}
	if err = db.Table("users").Count(&count).Error; err != nil || count != 3 {
		t.Fatal("orphan user created by collision", count, err)
	}
	if err = db.Exec("UPDATE authz_assignments SET deleted_at=NOW()").Error; err != nil {
		t.Fatal(err)
	}
	if _, err = Preflight(context.Background(), db, input, hasher); err == nil {
		t.Fatal("non-administrator provisioning allowed")
	}
}
