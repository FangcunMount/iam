package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseOptionsRequiresConfirmationForApply(t *testing.T) {
	for _, args := range [][]string{
		{"--apply"},
		{"--apply", "--confirm=wrong"},
	} {
		if _, err := parseOptions(args); err == nil {
			t.Fatalf("parseOptions(%v) should reject missing confirmation", args)
		}
	}

	opts, err := parseOptions([]string{"--apply", "--confirm=" + applyConfirmation, "--timeout=30s"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.apply || opts.timeout != 30*time.Second {
		t.Fatalf("options = %+v", opts)
	}
}

func TestParseOptionsRejectsConfirmationDuringDryRun(t *testing.T) {
	if _, err := parseOptions([]string{"--confirm=" + applyConfirmation}); err == nil {
		t.Fatal("dry-run must reject an apply confirmation")
	}
}

func TestDSNFromEnvironmentDoesNotIncludeSecretsInErrors(t *testing.T) {
	const secret = "mysql-password-sentinel"
	t.Setenv("IAM_MYSQL_DSN", "")
	t.Setenv("MYSQL_DSN", "")
	t.Setenv("MYSQL_HOST", "db")
	t.Setenv("MYSQL_USERNAME", "iam")
	t.Setenv("MYSQL_PASSWORD", secret)
	t.Setenv("MYSQL_DBNAME", "iam")
	t.Setenv("MYSQL_PORT", "invalid")
	_, err := dsnFromEnv()
	if err == nil {
		t.Fatal("invalid port should fail")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked secret: %v", err)
	}
}

func TestDSNFromEnvironmentRejectsMalformedDSNWithoutLeakingIt(t *testing.T) {
	const secret = "malformed-oneoff-dsn-secret"
	t.Setenv("IAM_MYSQL_DSN", secret+"@tcp(")
	_, err := dsnFromEnv()
	if err == nil {
		t.Fatal("malformed DSN should fail")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked malformed DSN: %v", err)
	}
}
