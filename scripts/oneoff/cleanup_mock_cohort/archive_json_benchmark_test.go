package main

import (
	"bytes"
	"encoding/json"
	"os"
	"runtime"
	"strconv"
	"testing"
)

// An identical underlying wire type deliberately takes the original slow path.
type archiveLegacyJSON archive

func benchmarkArchiveJSON(b *testing.B) []byte {
	b.Helper()
	if path := os.Getenv("IAM_MOCK_CLEANUP_ARCHIVE_BENCH_FILE"); path != "" {
		raw, err := readPrivate(path, os.Getenv("IAM_MOCK_CLEANUP_ARCHIVE_BENCH_SHA256"), maxDocument)
		if err != nil {
			b.Fatal(err)
		}
		var a archive
		if err = json.Unmarshal(raw, &a); err != nil {
			b.Fatal(err)
		}
		canonical, err := json.Marshal(&a)
		if err != nil || !bytes.Equal(bytes.TrimSpace(raw), canonical) {
			b.Fatal("fixed archive is not canonical")
		}
		b.Logf("fixed archive bytes=%d sha256=%s canonical=true", len(raw), rawSHA(raw))
		return raw
	}
	a := archiveJSONFixture()
	a.Tables = nil
	payload := bytes.Repeat([]byte{0, 1, 127, 255}, 96)
	for table, name := range tableOrder {
		im := tableImage{Name: name, DDL: "CREATE TABLE " + name + " (id BIGINT, value BLOB)", Columns: []column{{"id", "bigint", false}, {"value", "blob", false}}, Rows: []row{}}
		if name != "authz_assignments" {
			for i := 0; i < 6000; i++ {
				im.Rows = append(im.Rows, row{[]cell{{false, []byte(strconv.Itoa(100000 + table*6000 + i))}, {false, payload}}})
			}
		}
		a.Tables = append(a.Tables, im)
	}
	return encode(a)
}

func BenchmarkStrictArchiveJSON(b *testing.B) {
	raw := benchmarkArchiveJSON(b)
	runtime.GC()
	for _, name := range []string{"canonical", "legacy"} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(raw)))
			for i := 0; i < b.N; i++ {
				var err error
				if name == "canonical" {
					var a archive
					err = strictJSON(raw, &a)
				} else {
					var a archiveLegacyJSON
					err = strictJSON(raw, &a)
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
