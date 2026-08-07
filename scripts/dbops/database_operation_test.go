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
	"time"
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
	for _, want := range []string{"mysql_client=8.0.36", "connection=success", "size_mb=12.5", "tables=7", "backups=1"} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q: %s", want, output)
		}
	}
}

func TestIdentityRetirementRequiresFreshBackupConfirmationAndDropsOnlyLegacyTables(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	backupDir := filepath.Join(root, "backups")
	capture := filepath.Join(root, "identity-retirement.sql")
	requireNoError(t, os.MkdirAll(bin, 0o700))
	requireNoError(t, os.MkdirAll(backupDir, 0o700))
	writeExecutable(t, bin, "mysql", `#!/bin/sh
if [ "$1" = "--version" ]; then echo 'mysql  Ver 8.0.36'; exit 0; fi
case "$*" in
  *"MAX(version)"*) printf '18\t0\t1\n' ;;
  *"TABLE_NAME = 'children'"*"TABLE_TYPE = 'BASE TABLE'"*) printf '1\t1\n' ;;
  *"COUNT(*) FROM children"*) printf '64\t66\n' ;;
  *"KEY_COLUMN_USAGE"*) printf '%s\n' "${IAM_FAKE_DEPENDENCY_COUNT:-0}" ;;
  *) cat > "$IAM_FAKE_IDENTITY_RETIREMENT_CAPTURE"; printf '64\t66\n' ;;
esac
`)
	backupName := "iam_backup_20260807_170129.sql.gz"
	writeGzip(t, filepath.Join(backupDir, backupName), "verified backup fixture")

	base := map[string]string{
		"IAM_DB_OPS_OPERATION":                 "retire-identity-dry-run",
		"IAM_DB_OPS_BACKUP_DIR":                backupDir,
		"IAM_DB_OPS_BACKUP_NAME":               backupName,
		"IAM_FAKE_IDENTITY_RETIREMENT_CAPTURE": capture,
		"IAM_DB_OPS_MAX_BACKUP_AGE_SECONDS":    "7200",
	}
	output, err := runScript(t, bin, base)
	requireNoError(t, err)
	assertSafeOutput(t, output)
	for _, want := range []string{
		"state=eligible", "migration_version=18", "dependencies=0",
		"children_rows=64", "guardianships_rows=66", "reconciliation=waived_by_owner",
		"mode=dry-run result=success",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("identity dry-run output missing %q: %s", want, output)
		}
	}

	base["IAM_FAKE_DEPENDENCY_COUNT"] = "1"
	output, err = runScript(t, bin, base)
	if err == nil || !strings.Contains(output, "database dependencies still exist") {
		t.Fatalf("identity dry-run with dependency: err=%v output=%s", err, output)
	}
	assertSafeOutput(t, output)
	delete(base, "IAM_FAKE_DEPENDENCY_COUNT")

	base["IAM_DB_OPS_OPERATION"] = "retire-identity-apply"
	output, err = runScript(t, bin, base)
	if err == nil || !strings.Contains(output, "confirmation is invalid") {
		t.Fatalf("identity apply without confirmation: err=%v output=%s", err, output)
	}
	assertSafeOutput(t, output)

	base["IAM_DB_OPS_CONFIRMATION"] = "RETIRE_CHILDREN_GUARDIANSHIPS"
	output, err = runScript(t, bin, base)
	requireNoError(t, err)
	assertSafeOutput(t, output)
	for _, want := range []string{
		"mode=apply result=success", "children_rows_deleted=64",
		"guardianships_rows_deleted=66", "canonical_writes=0",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("identity apply output missing %q: %s", want, output)
		}
	}

	sql, err := os.ReadFile(capture)
	requireNoError(t, err)
	source := string(sql)
	if strings.Count(source, "DROP TABLE children, guardianships;") != 1 {
		t.Fatalf("identity retirement must contain one exact final legacy DROP: %s", source)
	}
	for _, forbidden := range []string{
		"INSERT INTO profiles", "UPDATE profiles", "DELETE FROM profiles",
		"INSERT INTO profile_links", "UPDATE profile_links", "DELETE FROM profile_links",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("identity retirement contains forbidden canonical write %q", forbidden)
		}
	}
}

func TestIdentityRetirementRejectsStaleBackup(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	backupDir := filepath.Join(root, "backups")
	requireNoError(t, os.MkdirAll(bin, 0o700))
	requireNoError(t, os.MkdirAll(backupDir, 0o700))
	writeExecutable(t, bin, "mysql", "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo 'mysql  Ver 8.0.36'; exit 0; fi\nexit 99\n")
	backupName := "iam_backup_20260807_170129.sql.gz"
	backupPath := filepath.Join(backupDir, backupName)
	writeGzip(t, backupPath, "stale backup fixture")
	stale := time.Now().Add(-3 * time.Hour)
	requireNoError(t, os.Chtimes(backupPath, stale, stale))

	output, err := runScript(t, bin, map[string]string{
		"IAM_DB_OPS_OPERATION":              "retire-identity-dry-run",
		"IAM_DB_OPS_BACKUP_DIR":             backupDir,
		"IAM_DB_OPS_BACKUP_NAME":            backupName,
		"IAM_DB_OPS_MAX_BACKUP_AGE_SECONDS": "7200",
	})
	if err == nil || !strings.Contains(output, "retirement backup is stale") {
		t.Fatalf("stale retirement backup: err=%v output=%s", err, output)
	}
	assertSafeOutput(t, output)
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
	if strings.Count(source, "script_path: scripts/dbops/legacy-retirement-preflight.sh") != 1 {
		t.Fatal("database status must run the checked-out legacy retirement preflight once")
	}
	for _, want := range []string{
		"image_sha:", "IAM_RETIREMENT_IMAGE_SHA", "Run Legacy Retirement Preflight",
		"IAM_DB_OPS_ALLOW_DOCKER_CLIENT", "IAM_RETIREMENT_ALLOW_DOCKER_CLIENT",
		"retirement_scope:", "IAM_RETIREMENT_SCOPE",
		"retirement_io_waiver:", "IAM_RETIREMENT_OWNER_IO_WAIVER",
		"retire-identity-dry-run", "retire-identity-apply",
		"performance-schema-status",
		"IAM_DB_OPS_CONFIRMATION", "confirmation:",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("database status preflight is missing %q", want)
		}
	}
	for _, forbidden := range []string{"apt-get", "apk add", "script: |", "SHOW TABLES", "Largest tables"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("database workflow contains forbidden inline/runtime behavior %q", forbidden)
		}
	}

	concurrency, err := os.ReadFile(filepath.Join(repo, ".github", "workflows", "concurrency-tests.yml"))
	requireNoError(t, err)
	for _, want := range []string{
		"Verify database backup, restore, and Identity retirement with MySQL 8",
		"IAM_DB_OPS_OPERATION=backup", "IAM_DB_OPS_OPERATION=restore",
		"IAM_DB_OPS_OPERATION=retire-identity-dry-run",
		"IAM_DB_OPS_OPERATION=retire-identity-apply",
	} {
		if !strings.Contains(string(concurrency), want) {
			t.Fatalf("MySQL workflow is missing %q", want)
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
