package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
)

func TestLegacySnapshotCodecRoundTripsProfileCandidate(t *testing.T) {
	candidate := domainsuggest.NewProfileCandidate(1, "张三", []string{"13800138000", "13900139000"}, 5)

	line := EncodeLegacyLine(candidate)
	if line != "张三|1|13800138000,13900139000|-|5" {
		t.Fatalf("line = %q", line)
	}

	got, ok := DecodeLegacyLine(line)
	if !ok {
		t.Fatalf("DecodeLegacyLine() ok = false")
	}
	if got.ProfileID != 1 || got.DisplayName != "张三" || len(got.Mobiles) != 2 || got.Weight != 5 {
		t.Fatalf("candidate = %#v", got)
	}
}

func TestFileSnapshotWriterKeepsLegacyLineFormat(t *testing.T) {
	dir := t.TempDir()
	writer := NewFileSnapshotWriter(dir)
	candidates := []domainsuggest.ProfileCandidate{
		domainsuggest.NewProfileCandidate(1, "张三", []string{"13800138000"}, 5),
		domainsuggest.NewProfileCandidate(2, "李四", nil, 3),
	}

	if err := writer.Write(context.Background(), candidates); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, snapshotFileName))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	want := "张三|1|13800138000|-|5\n李四|2||-|3"
	if string(data) != want {
		t.Fatalf("snapshot = %q, want %q", string(data), want)
	}
}
