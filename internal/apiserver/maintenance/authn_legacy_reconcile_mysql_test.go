package maintenance

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

func TestAuthNLegacyReconciliationMySQL(t *testing.T) {
	db := openAuthNReconcileMySQL(t)

	t.Run("dry-run and apply insert only missing canonical facts", func(t *testing.T) {
		resetAuthNReconcileSchema(t, db)
		insertLegacyAuthNFixture(t, db)

		dryRun, err := ReconcileAuthNLegacy(context.Background(), db, AuthNLegacyReconcileOptions{})
		if err != nil {
			t.Fatalf("dry-run reconciliation: %v", err)
		}
		if dryRun.RetirementEligible || dryRun.PlannedLoginIdentityInserts != 2 || dryRun.PlannedPasswordInserts != 1 {
			t.Fatalf("dry-run summary = %+v", dryRun)
		}
		assertAuthNRowCount(t, db, "auth_login_identities", 0)
		assertAuthNRowCount(t, db, "auth_credentials", 0)

		applied, err := ReconcileAuthNLegacy(context.Background(), db, AuthNLegacyReconcileOptions{Apply: true})
		if err != nil {
			t.Fatalf("apply reconciliation: %v", err)
		}
		if applied.RetirementEligible || !applied.VerificationRequired ||
			applied.AppliedLoginIdentityInserts != 2 || applied.AppliedPasswordInserts != 1 ||
			applied.RemainingLoginIdentityRows != 0 || applied.RemainingPasswordRows != 0 {
			t.Fatalf("apply summary = %+v", applied)
		}
		assertAuthNRowCount(t, db, "auth_login_identities", 2)
		assertAuthNRowCount(t, db, "auth_credentials", 1)
		verified, err := ReconcileAuthNLegacy(context.Background(), db, AuthNLegacyReconcileOptions{})
		if err != nil || !verified.RetirementEligible || verified.VerificationRequired {
			t.Fatalf("post-apply verification: err=%v summary=%+v", err, verified)
		}

		if _, err := db.Exec(`UPDATE auth_login_identities
SET status = 'disabled', version = 9
WHERE provider = 'username' AND realm = 'default' AND identifier = 'operator'`); err != nil {
			t.Fatalf("change canonical identity: %v", err)
		}
		if _, err := db.Exec(`UPDATE auth_credentials
SET material = ?, status = 'disabled', failed_attempts = 7, version = 11
WHERE type = 'password'`, []byte("canonical-password-hash")); err != nil {
			t.Fatalf("change canonical credential: %v", err)
		}

		rerun, err := ReconcileAuthNLegacy(context.Background(), db, AuthNLegacyReconcileOptions{Apply: true})
		if err != nil {
			t.Fatalf("rerun reconciliation: %v", err)
		}
		if rerun.RetirementEligible || !rerun.VerificationRequired ||
			rerun.AppliedLoginIdentityInserts != 0 || rerun.AppliedPasswordInserts != 0 {
			t.Fatalf("rerun summary = %+v", rerun)
		}
		var identityStatus, credentialStatus string
		var identityVersion, credentialVersion, failedAttempts int
		var material []byte
		if err := db.QueryRow(`SELECT status, version FROM auth_login_identities
WHERE provider = 'username' AND realm = 'default' AND identifier = 'operator'`).
			Scan(&identityStatus, &identityVersion); err != nil {
			t.Fatalf("query canonical identity: %v", err)
		}
		if err := db.QueryRow(`SELECT material, status, failed_attempts, version FROM auth_credentials
WHERE type = 'password'`).Scan(&material, &credentialStatus, &failedAttempts, &credentialVersion); err != nil {
			t.Fatalf("query canonical credential: %v", err)
		}
		if identityStatus != "disabled" || identityVersion != 9 || credentialStatus != "disabled" ||
			failedAttempts != 7 || credentialVersion != 11 || string(material) != "canonical-password-hash" {
			t.Fatalf("canonical mutable facts were overwritten: identity=%s/%d credential=%s/%d/%d material=%q",
				identityStatus, identityVersion, credentialStatus, failedAttempts, credentialVersion, material)
		}
	})

	t.Run("ownership conflict fails without canonical writes", func(t *testing.T) {
		resetAuthNReconcileSchema(t, db)
		insertLegacyAuthNFixture(t, db)
		if _, err := db.Exec(`INSERT INTO auth_login_identities
(id, user_id, provider, realm, identifier, status, linked_at)
VALUES (9001, 999, 'username', 'default', 'operator', 'active', NOW())`); err != nil {
			t.Fatalf("insert conflicting identity: %v", err)
		}

		dryRun, dryRunErr := ReconcileAuthNLegacy(context.Background(), db, AuthNLegacyReconcileOptions{})
		if !errors.Is(dryRunErr, ErrAuthNLegacyConflicts) || dryRun.AccountOwnerConflicts != 1 {
			t.Fatalf("dry-run error = %v, summary = %+v", dryRunErr, dryRun)
		}

		summary, err := ReconcileAuthNLegacy(context.Background(), db, AuthNLegacyReconcileOptions{Apply: true})
		if !errors.Is(err, ErrAuthNLegacyConflicts) {
			t.Fatalf("apply error = %v, summary = %+v", err, summary)
		}
		if summary.AccountOwnerConflicts != 1 || summary.HardConflicts == 0 || summary.RetirementEligible {
			t.Fatalf("conflict summary = %+v", summary)
		}
		assertAuthNRowCount(t, db, "auth_login_identities", 1)
		assertAuthNRowCount(t, db, "auth_credentials", 0)
	})

	t.Run("OAuth migration is marker scoped and resumable in bounded batches", func(t *testing.T) {
		resetAuthNReconcileSchema(t, db)
		if _, err := db.Exec(`INSERT INTO auth_accounts
(id, user_id, type, app_id, external_id, status)
VALUES (10, 100, 'mock-consumer', '', 'consumer', 1)`); err != nil {
			t.Fatalf("insert OAuth account fixture: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO auth_credentials_legacy
(id, account_id, type, idp, idp_identifier, app_id, status)
VALUES (20, 10, 'oauth_wx_minip', 'wechat', 'legacy-open-or-union', 'wx-app', 1)`); err != nil {
			t.Fatalf("insert OAuth credential fixture: %v", err)
		}

		first, err := ReconcileAuthNLegacy(context.Background(), db, AuthNLegacyReconcileOptions{
			Apply: true, BatchSize: 1,
		})
		if err != nil {
			t.Fatalf("first OAuth batch: %v", err)
		}
		if first.RetirementEligible || first.AppliedLoginIdentityInserts != 1 ||
			first.RemainingLoginIdentityRows != 1 {
			t.Fatalf("first OAuth batch summary = %+v", first)
		}
		assertAuthNRowCount(t, db, "auth_login_identities", 1)

		second, err := ReconcileAuthNLegacy(context.Background(), db, AuthNLegacyReconcileOptions{
			Apply: true, BatchSize: 1,
		})
		if err != nil {
			t.Fatalf("second OAuth batch: %v", err)
		}
		if second.RetirementEligible || !second.VerificationRequired ||
			second.AppliedLoginIdentityInserts != 1 || second.RemainingLoginIdentityRows != 0 {
			t.Fatalf("second OAuth batch summary = %+v", second)
		}
		assertAuthNRowCount(t, db, "auth_login_identities", 2)
		verified, err := ReconcileAuthNLegacy(context.Background(), db, AuthNLegacyReconcileOptions{})
		if err != nil || !verified.RetirementEligible || verified.VerificationRequired {
			t.Fatalf("OAuth post-apply verification: err=%v summary=%+v", err, verified)
		}
		var marker string
		if err := db.QueryRow(`SELECT JSON_UNQUOTE(JSON_EXTRACT(meta_json, '$.legacy_identifier_semantics'))
FROM auth_login_identities
WHERE provider = 'wechat_minip' AND realm = 'wx-app' AND identifier = 'legacy-open-or-union'`).Scan(&marker); err != nil {
			t.Fatalf("query migrated OAuth marker: %v", err)
		}
		if marker != "openid_or_unionid" {
			t.Fatalf("OAuth marker = %q", marker)
		}
	})

	t.Run("already retired is idempotently eligible", func(t *testing.T) {
		resetAuthNReconcileSchema(t, db)
		if _, err := db.Exec("DROP TABLE auth_credentials_legacy, auth_accounts"); err != nil {
			t.Fatalf("drop legacy tables: %v", err)
		}
		summary, err := ReconcileAuthNLegacy(context.Background(), db, AuthNLegacyReconcileOptions{})
		if err != nil {
			t.Fatalf("already-retired dry-run: %v", err)
		}
		if summary.State != "already_absent" || !summary.RetirementEligible {
			t.Fatalf("already-retired summary = %+v", summary)
		}
	})

	t.Run("empty wrong database never reports retirement eligible", func(t *testing.T) {
		resetAuthNReconcileSchema(t, db)
		if _, err := db.Exec("DROP TABLE auth_credentials_legacy, auth_accounts, auth_credentials, auth_login_identities"); err != nil {
			t.Fatalf("drop AuthN tables: %v", err)
		}
		summary, err := ReconcileAuthNLegacy(context.Background(), db, AuthNLegacyReconcileOptions{})
		if err == nil {
			t.Fatalf("empty wrong database was accepted: %+v", summary)
		}
		if summary.State != "canonical_schema_missing" || summary.RetirementEligible {
			t.Fatalf("wrong-database summary = %+v", summary)
		}
	})
}

func openAuthNReconcileMySQL(t *testing.T) *sql.DB {
	t.Helper()
	host := os.Getenv("MYSQL_HOST")
	if host == "" {
		t.Skip("MYSQL_HOST is required for MySQL AuthN reconciliation semantics")
	}
	port, err := strconv.Atoi(authNTestEnvOr("MYSQL_PORT", "3306"))
	if err != nil {
		t.Fatalf("parse MYSQL_PORT: %v", err)
	}
	config := mysql.NewConfig()
	config.User = authNTestEnvOr("MYSQL_USER", authNTestEnvOr("MYSQL_USERNAME", "iam"))
	config.Passwd = os.Getenv("MYSQL_PASSWORD")
	config.Net = "tcp"
	config.Addr = host + ":" + strconv.Itoa(port)
	config.DBName = authNTestEnvOr("MYSQL_DATABASE", authNTestEnvOr("MYSQL_DBNAME", "iam_test"))
	config.ParseTime = true
	config.Loc = time.UTC
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("ping MySQL: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DROP TABLE IF EXISTS auth_credentials_legacy, auth_accounts, auth_credentials, auth_login_identities")
		_ = db.Close()
	})
	return db
}

func resetAuthNReconcileSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	statements := []string{
		"DROP TABLE IF EXISTS auth_credentials_legacy, auth_accounts, auth_credentials, auth_login_identities",
		`CREATE TABLE auth_accounts (
id BIGINT UNSIGNED NOT NULL, user_id BIGINT UNSIGNED NOT NULL, type VARCHAR(32) NOT NULL,
app_id VARCHAR(64) NOT NULL DEFAULT '', external_id VARCHAR(128) NOT NULL,
scoped_tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0, unique_id VARCHAR(128) DEFAULT NULL,
profile JSON DEFAULT NULL, meta JSON DEFAULT NULL, status TINYINT NOT NULL DEFAULT 1,
created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
deleted_at DATETIME DEFAULT NULL, created_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0, deleted_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
version INT UNSIGNED NOT NULL DEFAULT 1, PRIMARY KEY (id),
UNIQUE KEY uk_type_app_external (type, app_id, external_id)) ENGINE=InnoDB`,
		`CREATE TABLE auth_credentials_legacy (
id BIGINT NOT NULL AUTO_INCREMENT, account_id BIGINT UNSIGNED NOT NULL, type VARCHAR(32) NOT NULL,
idp VARCHAR(32) DEFAULT NULL, idp_identifier VARCHAR(256) NOT NULL DEFAULT '', app_id VARCHAR(64) DEFAULT NULL,
material VARBINARY(512) DEFAULT NULL, algo VARCHAR(32) DEFAULT NULL, params_json VARBINARY(1024) DEFAULT NULL,
status TINYINT NOT NULL DEFAULT 1, failed_attempts INT NOT NULL DEFAULT 0, locked_until DATETIME DEFAULT NULL,
last_success_at DATETIME DEFAULT NULL, last_failure_at DATETIME DEFAULT NULL,
created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
deleted_at DATETIME DEFAULT NULL, created_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0, deleted_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
version INT UNSIGNED NOT NULL DEFAULT 1, PRIMARY KEY (id), UNIQUE KEY uk_account_type (account_id, type)) ENGINE=InnoDB`,
		`CREATE TABLE auth_login_identities (
id BIGINT UNSIGNED NOT NULL, user_id BIGINT UNSIGNED NOT NULL, provider VARCHAR(32) NOT NULL,
realm VARCHAR(128) NOT NULL DEFAULT '', identifier VARCHAR(255) NOT NULL, global_identifier VARCHAR(255) DEFAULT NULL,
status VARCHAR(32) NOT NULL, verified_at DATETIME DEFAULT NULL, linked_at DATETIME NOT NULL,
profile_json JSON DEFAULT NULL, meta_json JSON DEFAULT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at DATETIME DEFAULT NULL,
created_by BIGINT UNSIGNED NOT NULL DEFAULT 0, updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
deleted_by BIGINT UNSIGNED NOT NULL DEFAULT 0, version INT UNSIGNED NOT NULL DEFAULT 1,
PRIMARY KEY (id), UNIQUE KEY uk_provider_realm_identifier (provider, realm, identifier)) ENGINE=InnoDB`,
		`CREATE TABLE auth_credentials (
id BIGINT UNSIGNED NOT NULL, login_identity_id BIGINT UNSIGNED NOT NULL, type VARCHAR(32) NOT NULL,
material VARBINARY(4096) DEFAULT NULL, algo VARCHAR(64) DEFAULT NULL, params_json JSON DEFAULT NULL,
status VARCHAR(32) NOT NULL, failed_attempts INT NOT NULL DEFAULT 0, locked_until DATETIME DEFAULT NULL,
last_success_at DATETIME DEFAULT NULL, last_failure_at DATETIME DEFAULT NULL,
created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
deleted_at DATETIME DEFAULT NULL, created_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0, deleted_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
version INT UNSIGNED NOT NULL DEFAULT 1, PRIMARY KEY (id),
UNIQUE KEY uk_identity_type (login_identity_id, type)) ENGINE=InnoDB`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("reset AuthN reconciliation schema: %v", err)
		}
	}
}

func insertLegacyAuthNFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO auth_accounts
(id, user_id, type, app_id, external_id, unique_id, status)
VALUES (10, 100, 'opera', '', 'operator', NULL, 1)`); err != nil {
		t.Fatalf("insert legacy account: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO auth_credentials_legacy
(id, account_id, type, idp, idp_identifier, material, algo, status)
VALUES (20, 10, 'password', NULL, '', ?, 'bcrypt', 1),
       (21, 10, 'phone_otp', 'phone', '+8613800000000', NULL, NULL, 1)`, []byte("legacy-password-hash")); err != nil {
		t.Fatalf("insert legacy credentials: %v", err)
	}
}

func assertAuthNRowCount(t *testing.T, db *sql.DB, table string, want int) {
	t.Helper()
	if table != "auth_login_identities" && table != "auth_credentials" {
		t.Fatalf("unsupported count table %q", table)
	}
	var got int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s rows = %d, want %d", table, got, want)
	}
}

func authNTestEnvOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
