package maintenance

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeAndDisposeIAMLogs(t *testing.T) {
	root := t.TempDir()
	writeTestLog(t, filepath.Join(root, "app.log"), "refresh token cached secret-value\nGORM trace SELECT secret\n")
	writeGzipTestLog(t, filepath.Join(root, "warn.log.1.gz"), "token_hint=secret\n")
	writeTestLog(t, filepath.Join(root, "other.log"), "must stay\n")
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestLog(t, filepath.Join(root, "nested", "error.log"), "must stay\n")

	plan, err := AnalyzeLogDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	summary := plan.Summary()
	if summary.FileCount != 2 || summary.RefreshTokenLogMatches != 2 || summary.GORMSQLLogMatches != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if _, err := os.Stat(filepath.Join(root, "app.log")); err != nil {
		t.Fatalf("dry-run analysis removed file: %v", err)
	}
	encoded, err := MarshalLogDisposalSummary(summary)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"secret-value", "SELECT secret", "app.log"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("summary leaked %q: %s", forbidden, encoded)
		}
	}

	applied, err := plan.Dispose()
	if err != nil {
		t.Fatal(err)
	}
	if applied.DeletedFiles != 2 {
		t.Fatalf("deleted files = %d, want 2", applied.DeletedFiles)
	}
	for _, keep := range []string{"other.log", filepath.Join("nested", "error.log")} {
		if _, err := os.Stat(filepath.Join(root, keep)); err != nil {
			t.Fatalf("non-IAM file %q removed: %v", keep, err)
		}
	}
}

func TestAnalyzeLogDirectoryRejectsSymlinkCandidate(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	writeTestLog(t, target, "content")
	if err := os.Symlink(target, filepath.Join(root, "error.log")); err != nil {
		t.Fatal(err)
	}
	if _, err := AnalyzeLogDirectory(root); err == nil {
		t.Fatal("expected symlink rejection")
	}
}

func TestValidateProductionLogDirectoryRejectsOtherPaths(t *testing.T) {
	for _, path := range []string{"", "/", "/var/log/iam/../other", t.TempDir()} {
		if err := ValidateProductionLogDirectory(path); err == nil {
			t.Fatalf("expected %q to be rejected", path)
		}
	}
}

func TestLogDisposalPartialFailureIsReportedAndRetryable(t *testing.T) {
	root := t.TempDir()
	appLog := filepath.Join(root, "app.log")
	errorLog := filepath.Join(root, "error.log")
	writeTestLog(t, appLog, "app")
	writeTestLog(t, errorLog, "error")
	plan, err := AnalyzeLogDirectory(root)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(errorLog); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(appLog, errorLog); err != nil {
		t.Fatal(err)
	}
	summary, err := plan.Dispose()
	if err == nil {
		t.Fatal("expected candidate revalidation failure")
	}
	if summary.DeletedFiles != 1 {
		t.Fatalf("partial summary = %+v, want one completed deletion", summary)
	}

	if err := os.Remove(errorLog); err != nil {
		t.Fatal(err)
	}
	writeTestLog(t, errorLog, "replacement")
	retry, err := AnalyzeLogDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	retried, err := retry.Dispose()
	if err != nil {
		t.Fatal(err)
	}
	if retried.DeletedFiles != 1 {
		t.Fatalf("retry summary = %+v", retried)
	}
}

func writeTestLog(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeGzipTestLog(t *testing.T, path, content string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := gzip.NewWriter(file)
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
