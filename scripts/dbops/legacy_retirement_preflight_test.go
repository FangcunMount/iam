package dbops_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLegacyRetirementPreflightIsReadOnlyAndCoversRetirementEvidence(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(file), "legacy-retirement-preflight.sh")
	data, err := os.ReadFile(script)
	requireNoError(t, err)
	source := string(data)

	for _, required := range []string{
		"format_version=3",
		"schema_migrations",
		"performance_schema.table_io_waits_summary_by_table",
		"information_schema.KEY_COLUMN_USAGE",
		"information_schema.TRIGGERS",
		"information_schema.VIEWS",
		"information_schema.ROUTINES",
		"information_schema.EVENTS",
		"children_to_profiles",
		"guardianships_to_profile_links",
		"auth_accounts_to_login_identities",
		"legacy_credentials_to_authn",
		"immutable_conflict_rows",
		"mutable_status_divergences",
		"duplicate_source_rows",
		"password_material_mismatches",
		"invalid_supported_rows",
		"unsupported_rows",
		"oauth_unmapped_rows",
		"unknown_credential_rows",
		"schema_signature",
		"exact_rows",
		"eligibility",
		"zero_io_interpretation=not_proof_without_full_observation_window",
		"--defaults-extra-file=",
		"IAM_RETIREMENT_ALLOW_DOCKER_CLIENT",
		"mysql:8.0",
		"IAM_RETIREMENT_SCOPE",
		"IAM_RETIREMENT_OWNER_IO_WAIVER",
		"owner_io_waiver_allows",
		"schema_contract",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("preflight is missing evidence contract %q", required)
		}
	}
	upper := strings.ToUpper(source)
	for _, forbidden := range []string{
		"INSERT INTO",
		"DELETE FROM",
		"DROP TABLE",
		"TRUNCATE TABLE",
		"ALTER TABLE",
		"CREATE TABLE",
		"REPLACE INTO",
		"MYSQL_PWD",
	} {
		if strings.Contains(upper, forbidden) {
			t.Fatalf("read-only preflight contains forbidden token %q", forbidden)
		}
	}
}

func TestLegacyRetirementPreflightUsesPrivateCredentialsAndReturnsAggregates(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	tmp := filepath.Join(root, "tmp")
	requireNoError(t, os.MkdirAll(bin, 0o700))
	requireNoError(t, os.MkdirAll(tmp, 0o700))

	fake := `#!/bin/bash
set -eu
if [ "$1" = "--version" ]; then
  echo "mysql  Ver 8.0.36"
  exit 0
fi
defaults=""
for arg in "$@"; do
  case "$arg" in
    *db-ops-password-sentinel*) exit 80 ;;
    --defaults-extra-file=*) defaults="${arg#*=}" ;;
  esac
done
[ -n "$defaults" ]
[ "$(stat -c %a "$defaults" 2>/dev/null || stat -f %Lp "$defaults")" = "600" ]
grep -q "password=db-ops-password-sentinel" "$defaults"
case "$*" in
  *"SELECT VERSION()"*) printf "8.0.36\tMySQL Community Server\t2026-08-07T00:00:00Z\n" ;;
  *"MAX(version)"*) printf "21\t${FAKE_MIGRATION_DIRTY:-0}\t1\n" ;;
  *"MAX(TABLE_ROWS)"*) printf "1\t4\t4096\t2026-08-06T00:00:00Z\n" ;;
  *"@@performance_schema"*) printf "1\n" ;;
  *"performance_schema.global_status"*) printf "86400\n" ;;
  *"table_io_waits_summary_by_table"*)
    [ "${FAKE_IO_UNAVAILABLE:-0}" = "1" ] && exit 82
    printf "0\t0\t${FAKE_IO_READS:-0}\t0\n"
    ;;
  *"information_schema.KEY_COLUMN_USAGE"*) printf "0\t0\t0\t0\t0\t0\t0\t0\n" ;;
  *"TABLE_NAME = 'auth_credentials' AND COLUMN_NAME = 'account_id'"*) printf "0\n" ;;
  *"information_schema.COLUMNS"*) printf "12\t0123456789abcdef\n" ;;
  *"information_schema.STATISTICS"*) printf "4\tfedcba9876543210\n" ;;
  *"FROM children c"*) printf "legacy_rows=4\tmapped_rows=4\tmissing_rows=0\tmismatched_rows=0\n" ;;
  *"FROM guardianships g"*) printf "legacy_rows=3\tmapped_rows=3\tmissing_rows=0\tmismatched_rows=0\n" ;;
  *"FROM auth_credentials_legacy lc"*) printf "legacy_rows=3\tpassword_eligible_rows=1\tpassword_mapped_rows=1\tpassword_unmapped_rows=0\tpassword_material_mismatches=1\tpassword_duplicate_sources=0\tinvalid_password_rows=0\tphone_eligible_rows=1\tphone_identity_mapped_rows=1\tphone_blank_identifier_rows=0\tphone_orphan_account_rows=0\tphone_owner_conflicts=0\tphone_duplicate_sources=0\toauth_artifact_rows=1\toauth_redundant_rows=1\toauth_unmapped_rows=0\tunknown_credential_rows=0\n" ;;
  *"FROM auth_accounts a"*) printf "legacy_rows=2\tsupported_rows=2\tvalid_supported_rows=2\tinvalid_supported_rows=0\tunsupported_rows=0\tmapped_rows=2\tmissing_supported_rows=0\timmutable_conflict_rows=0\tduplicate_source_rows=0\tmutable_status_divergences=2\n" ;;
  *"information_schema.TABLES"*) printf "1\n" ;;
  *"SELECT COUNT(*) FROM "*) printf "4\n" ;;
  *) exit 81 ;;
esac
`
	mysql := filepath.Join(bin, "mysql")
	writeExecutable(t, bin, "mysql", fake)

	_, file, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(file), "legacy-retirement-preflight.sh")
	cmd := exec.Command("/bin/bash", script)
	cmd.Env = append(os.Environ(),
		"TMPDIR="+tmp,
		"IAM_RETIREMENT_MYSQL_BIN="+mysql,
		"IAM_RETIREMENT_ENVIRONMENT=staging",
		"IAM_RETIREMENT_COMMIT_SHA=0123456789abcdef",
		"IAM_RETIREMENT_IMAGE_SHA=sha256:abcdef",
		"MYSQL_HOST=db-host-sentinel",
		"MYSQL_PORT=3306",
		"MYSQL_USERNAME=iam-user-sentinel",
		"MYSQL_PASSWORD="+secretSentinel,
		"MYSQL_DBNAME=iam_test",
	)
	output, err := cmd.CombinedOutput()
	requireNoError(t, err)
	text := string(output)
	assertSafeOutput(t, text)
	for _, required := range []string{
		"query_mode=read_only_aggregate",
		"metadata\tenvironment\tstaging",
		"format_version=3",
		"migration\tschema_migrations\tpresent\tversion=21\tdirty=0",
		"candidate_table\tchildren\tpresent=1",
		"performance_schema\tstate=enabled",
		"table_io\tchildren\tcount_star=0",
		"dependency\tchildren\tfk=0",
		"exact_rows\tchildren\tstate=available\trows=4",
		"schema_signature\tchildren\tstate=available",
		"schema_contract\tchildren\texpected_columns=14",
		"parity\tchildren_to_profiles\tstate=available\tlegacy_rows=4",
		"password_material_mismatches=1",
		"eligibility\tschema_version\tstate=eligible",
		"eligibility\tauth_accounts\tstate=eligible",
		"eligibility\tauth_credentials_legacy\tstate=eligible",
		"eligibility\taudit_logs\tstate=eligible\trepository_gate=retire_unused_audit_tables",
		"legacy_retirement_preflight\tresult=success",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("preflight output missing %q:\n%s", required, text)
		}
	}
	leftovers, err := filepath.Glob(filepath.Join(tmp, "iam-retirement-*"))
	requireNoError(t, err)
	if len(leftovers) != 0 {
		t.Fatalf("preflight left credential/error files: %v", leftovers)
	}

	dirtyCmd := exec.Command("/bin/bash", script)
	dirtyCmd.Env = append(cmd.Env, "FAKE_MIGRATION_DIRTY=1")
	dirtyOutput, err := dirtyCmd.CombinedOutput()
	requireNoError(t, err)
	assertSafeOutput(t, string(dirtyOutput))
	if !strings.Contains(string(dirtyOutput), "eligibility\tschema_version\tstate=blocked\treason=schema_migrations_dirty") {
		t.Fatalf("dirty migration did not fail eligibility closed:\n%s", dirtyOutput)
	}

	ioCmd := exec.Command("/bin/bash", script)
	ioCmd.Env = append(cmd.Env, "FAKE_IO_READS=1", "IAM_RETIREMENT_OWNER_IO_WAIVER=platform_tables")
	ioOutput, err := ioCmd.CombinedOutput()
	requireNoError(t, err)
	assertSafeOutput(t, string(ioOutput))
	if !strings.Contains(string(ioOutput), "eligibility\tschema_version\tstate=blocked\treason=instantaneous_io_nonzero") {
		t.Fatalf("nonzero I/O did not fail eligibility closed:\n%s", ioOutput)
	}

	unavailableCmd := exec.Command("/bin/bash", script)
	unavailableCmd.Env = append(cmd.Env,
		"FAKE_IO_UNAVAILABLE=1",
		"IAM_RETIREMENT_SCOPE=schema_version",
	)
	unavailableOutput, err := unavailableCmd.CombinedOutput()
	requireNoError(t, err)
	assertSafeOutput(t, string(unavailableOutput))
	if !strings.Contains(string(unavailableOutput), "eligibility\tschema_version\tstate=blocked\treason=table_io_unavailable") {
		t.Fatalf("unavailable I/O without waiver did not fail closed:\n%s", unavailableOutput)
	}

	waiverCmd := exec.Command("/bin/bash", script)
	waiverCmd.Env = append(cmd.Env,
		"FAKE_IO_UNAVAILABLE=1",
		"IAM_RETIREMENT_SCOPE=schema_version",
		"IAM_RETIREMENT_OWNER_IO_WAIVER=platform_tables",
	)
	waiverOutput, err := waiverCmd.CombinedOutput()
	requireNoError(t, err)
	assertSafeOutput(t, string(waiverOutput))
	if !strings.Contains(string(waiverOutput), "eligibility\tschema_version\tstate=eligible\trepository_gate=retire_schema_version\tevidence=owner_io_waiver") {
		t.Fatalf("owner waiver did not admit schema_version with unavailable I/O:\n%s", waiverOutput)
	}

	authnWaiverCmd := exec.Command("/bin/bash", script)
	authnWaiverCmd.Env = append(cmd.Env,
		"FAKE_IO_UNAVAILABLE=1",
		"IAM_RETIREMENT_SCOPE=authn",
		"IAM_RETIREMENT_OWNER_IO_WAIVER=platform_tables",
	)
	authnWaiverOutput, err := authnWaiverCmd.CombinedOutput()
	requireNoError(t, err)
	assertSafeOutput(t, string(authnWaiverOutput))
	if !strings.Contains(string(authnWaiverOutput), "eligibility\tauth_accounts\tstate=blocked\treason=table_io_unavailable") {
		t.Fatalf("platform waiver unexpectedly admitted AuthN:\n%s", authnWaiverOutput)
	}

	auditWaiverCmd := exec.Command("/bin/bash", script)
	auditWaiverCmd.Env = append(cmd.Env,
		"FAKE_IO_UNAVAILABLE=1",
		"IAM_RETIREMENT_SCOPE=audit",
		"IAM_RETIREMENT_OWNER_IO_WAIVER=audit_tables",
	)
	auditWaiverOutput, err := auditWaiverCmd.CombinedOutput()
	requireNoError(t, err)
	assertSafeOutput(t, string(auditWaiverOutput))
	for _, table := range []string{"operation_logs", "audit_logs", "auth_token_audit"} {
		want := "eligibility\t" + table + "\tstate=eligible\trepository_gate=retire_unused_audit_tables\tevidence=owner_io_waiver"
		if !strings.Contains(string(auditWaiverOutput), want) {
			t.Fatalf("audit owner waiver did not admit %s with unavailable I/O:\n%s", table, auditWaiverOutput)
		}
	}

	identityCmd := exec.Command("/bin/bash", script)
	identityCmd.Env = append(cmd.Env, "IAM_RETIREMENT_SCOPE=identity")
	identityOutput, err := identityCmd.CombinedOutput()
	requireNoError(t, err)
	assertSafeOutput(t, string(identityOutput))
	if !strings.Contains(string(identityOutput), "scope=identity") || strings.Contains(string(identityOutput), "parity\tauth_accounts_to_login_identities") {
		t.Fatalf("identity scope executed an unrelated AuthN parity query:\n%s", identityOutput)
	}
}
