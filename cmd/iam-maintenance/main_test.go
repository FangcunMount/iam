package main

import (
	"bytes"
	"net"
	"strconv"
	"strings"
	"testing"

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

func TestRetiredAuthNMaintenanceSubcommandIsRejected(t *testing.T) {
	if err := run([]string{"reconcile-authn-legacy"}, &bytes.Buffer{}); err == nil || err.Error() != "unsupported maintenance subcommand" {
		t.Fatalf("retired AuthN subcommand was not rejected: %v", err)
	}
}
