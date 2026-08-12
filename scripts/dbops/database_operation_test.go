package dbops_test

import (
	"bytes"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const secretSentinel = "db-ops-password-sentinel"

func TestBackupIsAtomicPrivateAndRetained(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	backupDir := filepath.Join(root, "backups")
	requireNoError(t, os.MkdirAll(bin, 0o700))
	requireNoError(t, os.MkdirAll(backupDir, 0o700))
	writeExecutable(t, bin, "mysql", `#!/bin/sh
if [ "$1" = "--version" ]; then echo 'mysql  Ver 8.0.36'; exit 0; fi
echo 1
`)
	writeExecutable(t, bin, "mysqldump", `#!/bin/sh
if [ "$1" = "--version" ]; then echo 'mysqldump  Ver 8.0.36'; exit 0; fi
printf '%s\n' 'CREATE TABLE fixture (id INT);' 'INSERT INTO fixture VALUES (1);'
`)
	for _, stamp := range []string{"20260101_000001", "20260101_000002", "20260101_000003"} {
		writeGzip(t, filepath.Join(backupDir, "iam_backup_"+stamp+".sql.gz"), "old")
	}

	output, err := runScript(t, bin, map[string]string{
		"IAM_DB_OPS_OPERATION":  "backup",
		"IAM_DB_OPS_BACKUP_DIR": backupDir,
		"IAM_DB_OPS_TIMESTAMP":  "20260101_000004",
	})
	requireNoError(t, err)
	assertSafeOutput(t, output)
	if !strings.Contains(output, "operation=backup result=success") {
		t.Fatalf("missing success summary: %s", output)
	}

	matches, err := filepath.Glob(filepath.Join(backupDir, "iam_backup_*.sql.gz"))
	requireNoError(t, err)
	if len(matches) != 3 {
		t.Fatalf("backup count = %d, want 3", len(matches))
	}
	newPath := filepath.Join(backupDir, "iam_backup_20260101_000004.sql.gz")
	info, err := os.Stat(newPath)
	requireNoError(t, err)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("backup mode = %o, want 600", info.Mode().Perm())
	}
	assertValidGzip(t, newPath)
	partials, err := filepath.Glob(filepath.Join(backupDir, ".*.partial"))
	requireNoError(t, err)
	if len(partials) != 0 {
		t.Fatalf("partial backups remain: %v", partials)
	}
}

func TestBackupFailureDoesNotLeakOrPublishPartialFile(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	backupDir := filepath.Join(root, "backups")
	requireNoError(t, os.MkdirAll(bin, 0o700))
	requireNoError(t, os.MkdirAll(backupDir, 0o700))
	writeExecutable(t, bin, "mysql", "#!/bin/sh\necho 'mysql  Ver 8.0.36'\n")
	writeExecutable(t, bin, "mysqldump", `#!/bin/sh
if [ "$1" = "--version" ]; then echo 'mysqldump  Ver 8.0.36'; exit 0; fi
echo 'raw-dump-error-sentinel' >&2
exit 12
`)

	output, err := runScript(t, bin, map[string]string{
		"IAM_DB_OPS_OPERATION":  "backup",
		"IAM_DB_OPS_BACKUP_DIR": backupDir,
		"IAM_DB_OPS_TIMESTAMP":  "20260101_000005",
	})
	if err == nil {
		t.Fatal("backup error = nil, want failure")
	}
	assertSafeOutput(t, output)
	if strings.Contains(output, "raw-dump-error-sentinel") {
		t.Fatalf("output leaked raw client error: %s", output)
	}
	entries, err := os.ReadDir(backupDir)
	requireNoError(t, err)
	if len(entries) != 0 {
		t.Fatalf("failed backup left files: %v", entries)
	}
}

func TestBackupRejectsBrokenGzipAndExistingDestination(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	backupDir := filepath.Join(root, "backups")
	requireNoError(t, os.MkdirAll(bin, 0o700))
	requireNoError(t, os.MkdirAll(backupDir, 0o700))
	writeExecutable(t, bin, "mysql", "#!/bin/sh\necho 'mysql  Ver 8.0.36'\n")
	writeExecutable(t, bin, "mysqldump", `#!/bin/sh
if [ "$1" = "--version" ]; then echo 'mysqldump  Ver 8.0.36'; exit 0; fi
echo 'SQL payload'
`)
	writeExecutable(t, bin, "gzip", `#!/bin/sh
exit 19
`)

	output, err := runScript(t, bin, map[string]string{
		"IAM_DB_OPS_OPERATION":  "backup",
		"IAM_DB_OPS_BACKUP_DIR": backupDir,
		"IAM_DB_OPS_TIMESTAMP":  "20260101_000006",
	})
	if err == nil || !strings.Contains(output, "backup stream did not complete") {
		t.Fatalf("broken gzip result: err=%v output=%s", err, output)
	}
	assertSafeOutput(t, output)

	writeGzip(t, filepath.Join(backupDir, "iam_backup_20260101_000007.sql.gz"), "existing")
	output, err = runScript(t, bin, map[string]string{
		"IAM_DB_OPS_OPERATION":  "backup",
		"IAM_DB_OPS_BACKUP_DIR": backupDir,
		"IAM_DB_OPS_TIMESTAMP":  "20260101_000007",
	})
	if err == nil || !strings.Contains(output, "backup destination already exists") {
		t.Fatalf("existing destination result: err=%v output=%s", err, output)
	}
	assertSafeOutput(t, output)
}

func TestClientAndRestoreValidation(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	backupDir := filepath.Join(root, "backups")
	requireNoError(t, os.MkdirAll(bin, 0o700))
	requireNoError(t, os.MkdirAll(backupDir, 0o700))
	writeExecutable(t, bin, "mysql", "#!/bin/sh\necho 'mysql  Ver 5.7.44'\n")

	output, err := runScript(t, bin, map[string]string{
		"IAM_DB_OPS_OPERATION":  "status",
		"IAM_DB_OPS_BACKUP_DIR": backupDir,
	})
	if err == nil || !strings.Contains(output, "must be MySQL 8.x") {
		t.Fatalf("non-8 client result: err=%v output=%s", err, output)
	}
	assertSafeOutput(t, output)

	writeExecutable(t, bin, "mysql", "#!/bin/sh\necho 'mysql  Ver 8.0.36'\n")
	output, err = runScript(t, bin, map[string]string{
		"IAM_DB_OPS_OPERATION":   "restore",
		"IAM_DB_OPS_BACKUP_NAME": "../escape.sql.gz",
		"IAM_DB_OPS_BACKUP_DIR":  backupDir,
	})
	if err == nil || !strings.Contains(output, "backup name is invalid") {
		t.Fatalf("invalid restore result: err=%v output=%s", err, output)
	}
	assertSafeOutput(t, output)
}

func TestRestoreRejectsSymlinkAndCorruptArchive(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	backupDir := filepath.Join(root, "backups")
	realArchive := filepath.Join(root, "real.sql.gz")
	requireNoError(t, os.MkdirAll(bin, 0o700))
	requireNoError(t, os.MkdirAll(backupDir, 0o700))
	writeExecutable(t, bin, "mysql", "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo 'mysql  Ver 8.0.36'; exit 0; fi\ncat >/dev/null\n")
	writeGzip(t, realArchive, "SELECT 1;")

	symlinkName := "iam_backup_20260101_000009.sql.gz"
	requireNoError(t, os.Symlink(realArchive, filepath.Join(backupDir, symlinkName)))
	output, err := runScript(t, bin, map[string]string{
		"IAM_DB_OPS_OPERATION":   "restore",
		"IAM_DB_OPS_BACKUP_NAME": symlinkName,
		"IAM_DB_OPS_BACKUP_DIR":  backupDir,
	})
	if err == nil || !strings.Contains(output, "backup file is unavailable") {
		t.Fatalf("symlink restore result: err=%v output=%s", err, output)
	}
	assertSafeOutput(t, output)

	corruptName := "iam_backup_20260101_000010.sql.gz"
	requireNoError(t, os.WriteFile(filepath.Join(backupDir, corruptName), []byte("not-gzip"), 0o600))
	output, err = runScript(t, bin, map[string]string{
		"IAM_DB_OPS_OPERATION":   "restore",
		"IAM_DB_OPS_BACKUP_NAME": corruptName,
		"IAM_DB_OPS_BACKUP_DIR":  backupDir,
	})
	if err == nil || !strings.Contains(output, "backup integrity validation failed") {
		t.Fatalf("corrupt restore result: err=%v output=%s", err, output)
	}
	assertSafeOutput(t, output)
}

func TestMissingConfigurationAndClientFailClosed(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, "backups")
	requireNoError(t, os.MkdirAll(backupDir, 0o700))

	output, err := runScriptWithBaseEnv(t, nil, map[string]string{
		"IAM_DB_OPS_OPERATION":  "status",
		"IAM_DB_OPS_BACKUP_DIR": backupDir,
		"MYSQL_PASSWORD":        "",
	})
	if err == nil || !strings.Contains(output, "required configuration is missing") {
		t.Fatalf("missing configuration result: err=%v output=%s", err, output)
	}
	assertSafeOutput(t, output)

	output, err = runScriptWithBaseEnv(t, nil, map[string]string{
		"IAM_DB_OPS_OPERATION":  "status",
		"IAM_DB_OPS_BACKUP_DIR": backupDir,
		"IAM_DB_OPS_MYSQL_BIN":  "iam-mysql-does-not-exist",
	})
	if err == nil || !strings.Contains(output, "client is unavailable") {
		t.Fatalf("missing client result: err=%v output=%s", err, output)
	}
	assertSafeOutput(t, output)
}

func TestRestoreAndStatusReturnMetadataOnly(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	backupDir := filepath.Join(root, "backups")
	capture := filepath.Join(root, "restore.sql")
	requireNoError(t, os.MkdirAll(bin, 0o700))
	requireNoError(t, os.MkdirAll(backupDir, 0o700))
	writeExecutable(t, bin, "mysql", `#!/bin/sh
if [ "$1" = "--version" ]; then echo 'mysql  Ver 8.0.36'; exit 0; fi
case "$*" in
  *'SELECT 1;'*) echo '1' ;;
	  *'MAX(version)'*) printf '%b\n' "${IAM_FAKE_MIGRATION_STATE:-23\t0\t1}" ;;
	  *"TABLE_NAME IN ('children'"*) printf '%s\n' "${IAM_FAKE_RETIRED_TABLES:-0}" ;;
  *'IS_USED_LOCK'*) printf 'none\tfree\t-1\n' ;;
  *'COUNT(*)'*) echo '7' ;;
  *'SUM(data_length'*) echo '12.5' ;;
  *) cat > "$IAM_FAKE_RESTORE_CAPTURE" ;;
esac
`)
	backupName := "iam_backup_20260101_000008.sql.gz"
	writeGzip(t, filepath.Join(backupDir, backupName), "CREATE TABLE fixture (id INT);")

	output, err := runScript(t, bin, map[string]string{
		"IAM_DB_OPS_OPERATION":     "restore",
		"IAM_DB_OPS_BACKUP_NAME":   backupName,
		"IAM_DB_OPS_BACKUP_DIR":    backupDir,
		"IAM_FAKE_RESTORE_CAPTURE": capture,
	})
	requireNoError(t, err)
	assertSafeOutput(t, output)
	restored, err := os.ReadFile(capture)
	requireNoError(t, err)
	if !strings.Contains(string(restored), "CREATE TABLE fixture") {
		t.Fatalf("restore input was not delivered: %s", restored)
	}

	output, err = runScript(t, bin, map[string]string{
		"IAM_DB_OPS_OPERATION":  "status",
		"IAM_DB_OPS_BACKUP_DIR": backupDir,
	})
	requireNoError(t, err)
	assertSafeOutput(t, output)
	for _, want := range []string{"mysql_client=8.0.36", "connection=success", "size_mb=12.5", "tables=7", "backups=1", "schema_migrations=23", "retired_tables_present=0", "owner_state=none\tfree\t-1", "retirement guard: result=success"} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q: %s", want, output)
		}
	}

	for name, overrides := range map[string]map[string]string{
		"migration not current": {
			"IAM_FAKE_MIGRATION_STATE": "22\t0\t1",
		},
		"retired table returned": {
			"IAM_FAKE_RETIRED_TABLES": "1",
		},
	} {
		t.Run(name, func(t *testing.T) {
			caseEnv := map[string]string{
				"IAM_DB_OPS_OPERATION":  "status",
				"IAM_DB_OPS_BACKUP_DIR": backupDir,
			}
			for key, value := range overrides {
				caseEnv[key] = value
			}
			guardOutput, guardErr := runScript(t, bin, caseEnv)
			if guardErr == nil || !strings.Contains(guardOutput, "database operation failed") {
				t.Fatalf("retirement guard did not fail closed: err=%v output=%s", guardErr, guardOutput)
			}
			assertSafeOutput(t, guardOutput)
		})
	}
}

func TestPerformanceSchemaStatusIsReadOnlyAndSecretSafe(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	backupDir := filepath.Join(root, "backups")
	requireNoError(t, os.MkdirAll(bin, 0o700))
	requireNoError(t, os.MkdirAll(backupDir, 0o700))
	writeExecutable(t, bin, "mysql", `#!/bin/sh
if [ "$1" = "--version" ]; then echo 'mysql  Ver 8.0.36'; exit 0; fi
case "$*" in
  *"@@performance_schema"*) printf '0\t1\t0\t0\tmanaged_or_cloud\t8.0.36\n' ;;
  *"information_schema.TABLES"*"table_io_waits_summary_by_table"*) printf '1\t4\n' ;;
  *"SELECT COUNT(*) FROM performance_schema.table_io_waits_summary_by_table"*) printf '23\n' ;;
  *"SELECT COUNT(*) FROM sys.schema_table_statistics"*) printf '17\n' ;;
  *"SELECT @@opt_tablestat"*) printf '1\n' ;;
  *"SELECT COUNT(*) FROM information_schema.TABLE_STATISTICS"*) printf '19\n' ;;
  *"SHOW GRANTS"*) printf "GRANT USAGE ON *.* TO 'grant-user-sentinel'@'%%'\n" ;;
  *) exit 91 ;;
esac
`)

	output, err := runScript(t, bin, map[string]string{
		"IAM_DB_OPS_OPERATION":  "performance-schema-status",
		"IAM_DB_OPS_BACKUP_DIR": backupDir,
		"MYSQL_HOST":            "prod.mysql.rds.aliyuncs.com",
	})
	requireNoError(t, err)
	assertSafeOutput(t, output)
	for _, want := range []string{
		"result=success", "enabled=0", "persisted_globals_load=1",
		"persist_x509_subject_configured=0", "tls_active=0",
		"persist_privileges=not_visible", "server_flavor=managed_or_cloud",
		"endpoint_provider=aliyun_rds",
		"server_version=8.0.36", "next_action=configure_provider_or_server_startup",
		"table_io_contract=valid", "table_io_metadata_visible=1",
		"table_io_required_columns=4", "table_io_select=available",
		"sys_table_statistics_select=available",
		"rds_table_statistics_enabled=1", "rds_table_statistics_select=available",
		"restart_required=1",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("Performance Schema status output missing %q: %s", want, output)
		}
	}
	for _, forbidden := range []string{"grant-user-sentinel", "GRANT USAGE", "prod.mysql.rds.aliyuncs.com", "SET PERSIST", "RESTART"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("Performance Schema status output contains forbidden value %q: %s", forbidden, output)
		}
	}
}

func TestStatusFallsBackToOfficialMySQLContainerClient(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	backupDir := filepath.Join(root, "backups")
	requireNoError(t, os.MkdirAll(bin, 0o700))
	requireNoError(t, os.MkdirAll(backupDir, 0o700))
	writeExecutable(t, bin, "sudo", `#!/bin/sh
if [ "$1" = "-n" ]; then shift; fi
exec "$@"
`)
	writeExecutable(t, bin, "docker", `#!/bin/sh
set -eu
if [ "$1" = "info" ]; then exit 0; fi
[ "$1" = "run" ]
shift
while [ "$#" -gt 0 ]; do
  case "$1" in
    --rm|-i) shift ;;
    --network|--volume) shift 2 ;;
    *) shift; break ;;
  esac
done
client="$1"
shift
if [ "${1:-}" = "--version" ]; then
  printf '%s  Ver 8.0.36\n' "$client"
  exit 0
fi
case "$*" in
	  *"MAX(version)"*) printf '23\t0\t1\n' ;;
	  *"TABLE_NAME IN ('children'"*) printf '0\n' ;;
	  *"IS_USED_LOCK"*) printf 'none\tfree\t-1\n' ;;
	  *"SUM(data_length"*) printf '12.5\n' ;;
  *"COUNT(*)"*) printf '7\n' ;;
  *) printf '1\n' ;;
esac
`)

	output, err := runScriptWithBaseEnv(t, []string{
		"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"),
	}, map[string]string{
		"IAM_DB_OPS_OPERATION":           "status",
		"IAM_DB_OPS_BACKUP_DIR":          backupDir,
		"IAM_DB_OPS_MYSQL_BIN":           "iam-mysql-missing",
		"IAM_DB_OPS_MYSQLDUMP_BIN":       "iam-mysqldump-missing",
		"IAM_DB_OPS_ALLOW_DOCKER_CLIENT": "1",
		"IAM_DB_OPS_DOCKER_BIN":          filepath.Join(bin, "docker"),
		"IAM_DB_OPS_SUDO_BIN":            filepath.Join(bin, "sudo"),
	})
	requireNoError(t, err)
	assertSafeOutput(t, output)
	for _, want := range []string{"mysql_client=8.0.36", "connection=success", "size_mb=12.5", "tables=7"} {
		if !strings.Contains(output, want) {
			t.Fatalf("container fallback output missing %q: %s", want, output)
		}
	}
}
func TestWorkflowUsesSingleCheckedOutScriptAndMySQLIntegration(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	workflow, err := os.ReadFile(filepath.Join(repo, ".github", "workflows", "db-ops.yml"))
	requireNoError(t, err)
	source := string(workflow)
	if strings.Count(source, "uses: actions/checkout@v6") != 4 {
		t.Fatal("every database operation job must checkout the repository script")
	}
	if strings.Count(source, "script_path: scripts/dbops/database-operation.sh") != 4 {
		t.Fatal("every database operation job must use the single script_path")
	}
	for _, want := range []string{
		"backup", "restore", "status", "performance-schema-status",
		"IAM_DB_OPS_ALLOW_DOCKER_CLIENT",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("database workflow is missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"apt-get", "apk add", "script: |", "SHOW TABLES", "Largest tables",
		"legacy-retirement-preflight", "retire-identity", "reconcile-authn", "authn_batch_size",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("database workflow contains retired or inline behavior %q", forbidden)
		}
	}

	concurrency, err := os.ReadFile(filepath.Join(repo, ".github", "workflows", "concurrency-tests.yml"))
	requireNoError(t, err)
	for _, want := range []string{
		"Verify database backup, restore, and status with MySQL 8",
		"Run retirement migrations and full-chain tests",
		"TestRetireUnusedPlatformTablesMigrationMySQL",
		"TestRetireUnusedAuditTablesMigrationMySQL",
		"TestFullMigrationChainAndBootstrapMySQL",
		"IAM_DB_OPS_OPERATION=backup", "IAM_DB_OPS_OPERATION=restore",
		"IAM_DB_OPS_OPERATION=status",
	} {
		if !strings.Contains(string(concurrency), want) {
			t.Fatalf("MySQL workflow is missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"TestAuthNLegacyReconciliationMySQL",
		"TestLegacyRetirementPreflightAuthNMySQL",
		"IAM_DB_OPS_OPERATION=retire-identity",
	} {
		if strings.Contains(string(concurrency), forbidden) {
			t.Fatalf("MySQL workflow still contains retired operation %q", forbidden)
		}
	}
}

func runScript(t *testing.T, bin string, overrides map[string]string) (string, error) {
	t.Helper()
	overrides["IAM_DB_OPS_MYSQL_BIN"] = filepath.Join(bin, "mysql")
	overrides["IAM_DB_OPS_MYSQLDUMP_BIN"] = filepath.Join(bin, "mysqldump")
	return runScriptWithBaseEnv(t, []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH")}, overrides)
}

func runScriptWithBaseEnv(t *testing.T, extra []string, overrides map[string]string) (string, error) {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(file), "database-operation.sh")
	cmd := exec.Command("/bin/bash", script)
	cmd.Env = append(os.Environ(), extra...)
	base := map[string]string{
		"MYSQL_HOST":     "db-host-sentinel",
		"MYSQL_PORT":     "3306",
		"MYSQL_USERNAME": "iam-user-sentinel",
		"MYSQL_PASSWORD": secretSentinel,
		"MYSQL_DBNAME":   "iam_test",
	}
	for key, value := range overrides {
		base[key] = value
	}
	for key, value := range base {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	return output.String(), err
}

func writeExecutable(t *testing.T, dir, name, content string) {
	t.Helper()
	requireNoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o700))
}

func writeGzip(t *testing.T, path, content string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	requireNoError(t, err)
	writer := gzip.NewWriter(file)
	_, err = writer.Write([]byte(content))
	requireNoError(t, err)
	requireNoError(t, writer.Close())
	requireNoError(t, file.Close())
}

func assertValidGzip(t *testing.T, path string) {
	t.Helper()
	file, err := os.Open(path)
	requireNoError(t, err)
	reader, err := gzip.NewReader(file)
	requireNoError(t, err)
	requireNoError(t, reader.Close())
	requireNoError(t, file.Close())
}

func assertSafeOutput(t *testing.T, output string) {
	t.Helper()
	for _, forbidden := range []string{secretSentinel, "db-host-sentinel", "iam-user-sentinel", "raw-dump-error-sentinel", "CREATE TABLE"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("output contains sensitive marker %q: %s", forbidden, output)
		}
	}
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
