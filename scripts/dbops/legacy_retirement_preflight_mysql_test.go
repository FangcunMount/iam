package dbops_test

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

func TestLegacyRetirementPreflightAuthNMySQL(t *testing.T) {
	host := os.Getenv("MYSQL_HOST")
	password := os.Getenv("MYSQL_PASSWORD")
	if host == "" || password == "" {
		t.Skip("MYSQL_HOST and non-empty MYSQL_PASSWORD are required for AuthN preflight semantics")
	}
	mysqlBinary, err := exec.LookPath(authNPreflightEnvOr("IAM_RETIREMENT_MYSQL_BIN", "mysql"))
	if err != nil {
		t.Skip("MySQL client is required for AuthN preflight semantics")
	}
	port, err := strconv.Atoi(authNPreflightEnvOr("MYSQL_PORT", "3306"))
	if err != nil {
		t.Fatalf("parse MYSQL_PORT: %v", err)
	}
	username := authNPreflightEnvOr("MYSQL_USERNAME", authNPreflightEnvOr("MYSQL_USER", "root"))
	database := authNPreflightEnvOr("MYSQL_DATABASE", authNPreflightEnvOr("MYSQL_DBNAME", "iam_authn_preflight"))
	config := mysql.NewConfig()
	config.User = username
	config.Passwd = password
	config.Net = "tcp"
	config.Addr = host + ":" + strconv.Itoa(port)
	config.DBName = database
	config.ParseTime = true
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.Ping(); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}
	resetAuthNPreflightMySQL(t, db)
	t.Cleanup(func() {
		_, _ = db.Exec("DROP TABLE IF EXISTS auth_credentials_legacy, auth_accounts, auth_credentials, auth_login_identities, schema_migrations")
	})

	ioReset := true
	if _, err := db.Exec("TRUNCATE TABLE performance_schema.table_io_waits_summary_by_table"); err != nil {
		ioReset = false
	}

	_, file, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(file), "legacy-retirement-preflight.sh")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "/bin/bash", script)
	command.Env = append(os.Environ(),
		"IAM_RETIREMENT_MYSQL_BIN="+mysqlBinary,
		"IAM_RETIREMENT_ENVIRONMENT=mysql-fixture",
		"IAM_RETIREMENT_COMMIT_SHA=mysql-fixture",
		"IAM_RETIREMENT_IMAGE_SHA=mysql-fixture",
		"IAM_RETIREMENT_SCOPE=authn",
		"MYSQL_HOST="+host,
		"MYSQL_PORT="+strconv.Itoa(port),
		"MYSQL_USERNAME="+username,
		"MYSQL_PASSWORD="+password,
		"MYSQL_DBNAME="+database,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run AuthN preflight: %v\n%s", err, output)
	}
	text := string(output)
	assertSafeOutput(t, text)
	for _, want := range []string{
		"legacy_retirement_preflight\tformat_version=3",
		"parity\tauth_accounts_to_login_identities\tstate=available",
		"immutable_conflict_rows=0",
		"mutable_status_divergences=1",
		"parity\tlegacy_credentials_to_authn\tstate=available",
		"password_material_mismatches=1",
		"password_duplicate_sources=0",
		"phone_owner_conflicts=0",
		"oauth_wx_minip_rows=1",
		"oauth_wechat_minip_account_rows=1",
		"oauth_provider_mismatch_rows=0",
		"oauth_identity_missing_rows=0",
		"legacy_retirement_preflight\tresult=success",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("AuthN preflight output missing %q:\n%s", want, text)
		}
	}
	if ioReset {
		for _, table := range []string{"auth_accounts", "auth_credentials_legacy"} {
			want := "eligibility\t" + table + "\tstate=eligible"
			if !strings.Contains(text, want) {
				t.Fatalf("AuthN preflight output missing %q:\n%s", want, text)
			}
		}
	}
}

func resetAuthNPreflightMySQL(t *testing.T, db *sql.DB) {
	t.Helper()
	statements := []string{
		"DROP TABLE IF EXISTS auth_credentials_legacy, auth_accounts, auth_credentials, auth_login_identities, schema_migrations",
		`CREATE TABLE schema_migrations (version BIGINT NOT NULL, dirty BOOLEAN NOT NULL) ENGINE=InnoDB`,
		`INSERT INTO schema_migrations VALUES (22, 0)`,
		`CREATE TABLE auth_accounts (
id BIGINT UNSIGNED NOT NULL, user_id BIGINT UNSIGNED NOT NULL, type VARCHAR(32) NOT NULL,
app_id VARCHAR(64) NOT NULL DEFAULT '', external_id VARCHAR(128) NOT NULL,
scoped_tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0, unique_id VARCHAR(128) DEFAULT NULL,
profile JSON DEFAULT NULL, meta JSON DEFAULT NULL, status TINYINT NOT NULL DEFAULT 1,
created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
deleted_at DATETIME DEFAULT NULL, created_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0, deleted_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
version INT UNSIGNED NOT NULL DEFAULT 1, PRIMARY KEY (id),
UNIQUE KEY uk_type_app_external (type, app_id, external_id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE auth_credentials_legacy (
id BIGINT NOT NULL AUTO_INCREMENT, account_id BIGINT UNSIGNED NOT NULL, type VARCHAR(32) NOT NULL,
idp VARCHAR(32) DEFAULT NULL, idp_identifier VARCHAR(256) NOT NULL DEFAULT '', app_id VARCHAR(64) DEFAULT NULL,
material VARBINARY(512) DEFAULT NULL, algo VARCHAR(32) DEFAULT NULL, params_json VARBINARY(1024) DEFAULT NULL,
status TINYINT NOT NULL DEFAULT 1, failed_attempts INT NOT NULL DEFAULT 0, locked_until DATETIME DEFAULT NULL,
last_success_at DATETIME DEFAULT NULL, last_failure_at DATETIME DEFAULT NULL,
created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
deleted_at DATETIME DEFAULT NULL, created_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0, deleted_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
version INT UNSIGNED NOT NULL DEFAULT 1, PRIMARY KEY (id),
UNIQUE KEY uk_account_type (account_id, type)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE auth_login_identities (
id BIGINT UNSIGNED NOT NULL, user_id BIGINT UNSIGNED NOT NULL, provider VARCHAR(32) NOT NULL,
realm VARCHAR(128) NOT NULL DEFAULT '', identifier VARCHAR(255) NOT NULL, global_identifier VARCHAR(255) DEFAULT NULL,
status VARCHAR(32) NOT NULL, verified_at DATETIME DEFAULT NULL, linked_at DATETIME NOT NULL,
profile_json JSON DEFAULT NULL, meta_json JSON DEFAULT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at DATETIME DEFAULT NULL,
created_by BIGINT UNSIGNED NOT NULL DEFAULT 0, updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
deleted_by BIGINT UNSIGNED NOT NULL DEFAULT 0, version INT UNSIGNED NOT NULL DEFAULT 1,
PRIMARY KEY (id), UNIQUE KEY uk_provider_realm_identifier (provider, realm, identifier))
ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE auth_credentials (
id BIGINT UNSIGNED NOT NULL, login_identity_id BIGINT UNSIGNED NOT NULL, type VARCHAR(32) NOT NULL,
material VARBINARY(4096) DEFAULT NULL, algo VARCHAR(64) DEFAULT NULL, params_json JSON DEFAULT NULL,
status VARCHAR(32) NOT NULL, failed_attempts INT NOT NULL DEFAULT 0, locked_until DATETIME DEFAULT NULL,
last_success_at DATETIME DEFAULT NULL, last_failure_at DATETIME DEFAULT NULL,
created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
deleted_at DATETIME DEFAULT NULL, created_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0, deleted_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
version INT UNSIGNED NOT NULL DEFAULT 1, PRIMARY KEY (id),
UNIQUE KEY uk_identity_type (login_identity_id, type)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`INSERT INTO auth_accounts (id, user_id, type, app_id, external_id, unique_id, status)
VALUES (10, 100, 'opera', '', 'operator', NULL, 1),
       (11, 101, 'wc-minip', 'wx-app', 'openid', 'unionid', 1)`,
		`INSERT INTO auth_credentials_legacy
(id, account_id, type, idp, idp_identifier, material, algo, status)
VALUES (20, 10, 'password', NULL, '', 'legacy-password-hash', 'bcrypt', 1),
	   (21, 10, 'phone_otp', 'phone', '+8613800000000', NULL, NULL, 1),
	   (22, 11, 'oauth_wx_minip', 'wechat', 'unionid', NULL, NULL, 1)`,
		`INSERT INTO auth_login_identities
(id, user_id, provider, realm, identifier, global_identifier, status, linked_at, version)
VALUES (110, 100, 'username', 'default', 'operator', NULL, 'disabled', NOW(), 9),
	   (111, 100, 'phone', 'global', '+8613800000000', NULL, 'active', NOW(), 1),
	   (112, 101, 'wechat_minip', 'wx-app', 'openid', 'unionid', 'active', NOW(), 1)`,
		`INSERT INTO auth_credentials
(id, login_identity_id, type, material, algo, status, failed_attempts, version)
VALUES (220, 110, 'password', 'canonical-password-hash', 'bcrypt', 'disabled', 7, 11)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("reset AuthN preflight fixture: %v", err)
		}
	}
}

func authNPreflightEnvOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
