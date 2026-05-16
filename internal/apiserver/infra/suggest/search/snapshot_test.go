package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
)

func TestSnapshotCodecRoundTripsProfileSearchTerm(t *testing.T) {
	term := domainsuggest.NewProfileSearchTerm(1, "张三", []string{"13800138000", "13900139000"}, 5, 9, 3, []int64{101, 102})

	line := EncodeSnapshotLine(term)
	want := "张三|1|13800138000,13900139000|9|3|101,102|5"
	if line != want {
		t.Fatalf("line = %q, want %q", line, want)
	}

	got, ok := DecodeSnapshotLine(line)
	if !ok {
		t.Fatalf("DecodeSnapshotLine() ok = false")
	}
	if got.ProfileID != 1 || got.DisplayName != "张三" || len(got.Mobiles) != 2 || got.Weight != 5 {
		t.Fatalf("term = %#v", got)
	}
	if got.TenantID != 9 || got.OrgID != 3 || len(got.OwnerOperatorIDs) != 2 {
		t.Fatalf("scope fields = %#v", got)
	}
}

func TestSnapshotCodecReadsLegacyLines(t *testing.T) {
	line := "张三|1|13800138000,13900139000|-|5"
	got, ok := DecodeSnapshotLine(line)
	if !ok {
		t.Fatalf("DecodeSnapshotLine() ok = false")
	}
	if got.ProfileID != 1 || got.TenantID != 0 || got.OrgID != 0 {
		t.Fatalf("term = %#v", got)
	}
}

func TestFileSnapshotWriterWritesSnapshotLines(t *testing.T) {
	dir := t.TempDir()
	writer := NewFileSnapshotWriter(dir)
	terms := []domainsuggest.ProfileSearchTerm{
		domainsuggest.NewProfileSearchTerm(1, "张三", []string{"13800138000"}, 5, 0, 0, nil),
		domainsuggest.NewProfileSearchTerm(2, "李四", nil, 3, 0, 0, nil),
	}

	if err := writer.Write(context.Background(), terms); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, snapshotFileName))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	want := "张三|1|13800138000|0|0||5\n李四|2||0|0||3"
	if string(data) != want {
		t.Fatalf("snapshot = %q, want %q", string(data), want)
	}
}
