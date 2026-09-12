package main

import (
	"bytes"
	"errors"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/scopemigrate"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScopeMutationArgumentsFailBeforeDatabase(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"apply", "--migration-id", "scope-v1", "--report", "/tmp/report"}, {"status", "--migration-id", "scope-v1", "--report", "relative"}, {"rollback", "--migration-id", "scope-v1", "--report", "/tmp/report", "--writes-stopped", "--actor-id", "09", "--fingerprint", strings.Repeat("a", 64)}} {
		if _, err := parseScopeOptions(args); err == nil {
			t.Fatal("invalid command accepted")
		}
	}
	if _, err := parseScopeOptions([]string{"apply", "--migration-id", "scope-v1", "--report", "/private/report", "--writes-stopped", "--actor-id", "9", "--fingerprint", strings.Repeat("a", 64)}); err != nil {
		t.Fatal(err)
	}
}
func TestScopeReportIsPrivateAndLogsAreSummaryOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "report.json")
	var output bytes.Buffer
	calls := 0
	args := []string{"preflight", "--migration-id", "scope-v1", "--report", path}
	execute := func(scopeOptions) (scopemigrate.Report, error) {
		calls++
		return scopemigrate.Report{State: "pending", NextAction: "resolve_issues", Plan: &scopemigrate.Plan{Issues: []string{"user:private-id"}}}, errors.New("private-id mapping conflict")
	}
	if err := runScopeMigrationWith(args, &output, execute); err == nil {
		t.Fatal("failed preflight returned success")
	}
	if strings.Contains(output.String(), "private-id") {
		t.Fatal("private details in standard output")
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(data, []byte("private-id")) {
		t.Fatal("missing private evidence")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("report is not private")
	}
	if err := runScopeMigrationWith(args, &output, execute); err == nil || calls != 1 {
		t.Fatal("overwrote report or executed before checking output")
	}
}
func TestScopeReportRejectsPublicDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if f, err := openScopeReport(filepath.Join(dir, "report")); err == nil {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
		t.Fatal("public report directory accepted")
	}
}
