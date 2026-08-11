package main

import (
	"bytes"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/FangcunMount/iam/v2/internal/apiserver/maintenance"
	"github.com/alicebob/miniredis/v2"
)

func TestPurgeRefreshTokensRequiresConfirmation(t *testing.T) {
	for _, args := range [][]string{
		{"purge-refresh-tokens", "--apply"},
		{"purge-refresh-tokens", "--apply", "--confirm=wrong"},
	} {
		if err := run(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("run(%v) should reject missing confirmation", args)
		}
	}
}

func TestPurgeRefreshTokensRejectsClusterWithoutLeakingConfig(t *testing.T) {
	t.Setenv("IAM_APISERVER_REDIS_CACHE_ENABLE_CLUSTER", "true")
	const secret = "redis-password-sentinel"
	t.Setenv("IAM_APISERVER_REDIS_CACHE_PASSWORD", secret)
	err := run([]string{"purge-refresh-tokens"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected cluster rejection")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked redis password: %v", err)
	}
}

func TestPurgeRefreshTokensDryRunAndApplyOutputCountsOnly(t *testing.T) {
	mr := miniredis.RunT(t)
	host, portText, err := net.SplitHostPort(mr.Addr())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("IAM_APISERVER_REDIS_CACHE_HOST", host)
	t.Setenv("IAM_APISERVER_REDIS_CACHE_PORT", strconv.Itoa(port))
	const sentinel = "maintenance-refresh-secret"
	if err := mr.Set("refresh_token:"+sentinel, "value"); err != nil {
		t.Fatal(err)
	}
	if err := mr.Set("session:keep", "value"); err != nil {
		t.Fatal(err)
	}

	var dryRun bytes.Buffer
	if err := run([]string{"purge-refresh-tokens"}, &dryRun); err != nil {
		t.Fatal(err)
	}
	if !mr.Exists("refresh_token:" + sentinel) {
		t.Fatal("dry-run removed token")
	}
	if strings.Contains(dryRun.String(), sentinel) || strings.Contains(dryRun.String(), "refresh_token:") {
		t.Fatalf("dry-run leaked key: %s", dryRun.String())
	}

	var applied bytes.Buffer
	if err := run([]string{
		"purge-refresh-tokens",
		"--apply",
		"--confirm=" + purgeConfirmation,
		"--batch-size=1",
	}, &applied); err != nil {
		t.Fatal(err)
	}
	if mr.Exists("refresh_token:"+sentinel) || !mr.Exists("session:keep") {
		t.Fatal("apply did not isolate refresh token keyspace")
	}
	if !strings.Contains(applied.String(), `"deleted":1`) {
		t.Fatalf("apply output = %s", applied.String())
	}
}

func TestDisposeSensitiveLogsRequiresConfirmationAndCanonicalPath(t *testing.T) {
	if err := run([]string{"dispose-sensitive-logs", "--apply"}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected confirmation failure")
	}
	if err := run([]string{"dispose-sensitive-logs", "--path=/"}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected unsafe path failure")
	}
}

func TestReconcileAuthNLegacyRequiresConfirmationBeforeDatabaseAccess(t *testing.T) {
	for _, args := range [][]string{
		{"reconcile-authn-legacy", "--apply"},
		{"reconcile-authn-legacy", "--apply", "--confirm=wrong"},
		{"reconcile-authn-legacy", "--apply", "--confirm=" + authNLegacyConfirmation, "--require-eligible"},
		{"reconcile-authn-legacy", "--confirm=" + authNLegacyConfirmation},
	} {
		if err := run(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("run(%v) should reject invalid confirmation", args)
		}
	}
}

func TestReconcileAuthNLegacyConfigurationErrorsDoNotLeakPassword(t *testing.T) {
	const secret = "authn-reconcile-password-sentinel"
	t.Setenv("IAM_MYSQL_DSN", "")
	t.Setenv("MYSQL_DSN", "")
	t.Setenv("IAM_APISERVER_MYSQL_HOST", "")
	t.Setenv("IAM_APISERVER_MYSQL_USERNAME", "")
	t.Setenv("IAM_APISERVER_MYSQL_PASSWORD", "")
	t.Setenv("IAM_APISERVER_MYSQL_DATABASE", "")
	t.Setenv("MYSQL_HOST", "db.internal")
	t.Setenv("MYSQL_USERNAME", "iam")
	t.Setenv("MYSQL_PASSWORD", secret)
	t.Setenv("MYSQL_DBNAME", "iam")
	t.Setenv("MYSQL_PORT", "invalid")

	err := run([]string{"reconcile-authn-legacy"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("invalid MySQL port should fail")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked MySQL password: %v", err)
	}
}

func TestReconcileAuthNLegacyRejectsMalformedDSNWithoutLeakingIt(t *testing.T) {
	const secret = "malformed-dsn-secret-sentinel"
	t.Setenv("IAM_MYSQL_DSN", secret+"@tcp(")
	_, err := mysqlDSNFromEnvironment()
	if err == nil {
		t.Fatal("malformed DSN should fail")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked malformed DSN: %v", err)
	}
}

func TestAuthNReconcileExitErrorRequiresRetirementEligibility(t *testing.T) {
	notEligible := maintenance.AuthNLegacyReconcileSummary{RetirementEligible: false}
	if err := authNReconcileExitError(notEligible, nil, false); err != nil {
		t.Fatalf("ordinary dry-run should report its plan without failing: %v", err)
	}
	if err := authNReconcileExitError(notEligible, nil, true); err == nil {
		t.Fatal("verify mode accepted a non-eligible summary")
	}
	eligible := maintenance.AuthNLegacyReconcileSummary{RetirementEligible: true}
	if err := authNReconcileExitError(eligible, nil, true); err != nil {
		t.Fatalf("verify mode rejected an eligible summary: %v", err)
	}
	if err := authNReconcileExitError(eligible, maintenance.ErrAuthNLegacyConflicts, true); err == nil {
		t.Fatal("verify mode accepted hard conflicts")
	}
}
