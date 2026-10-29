package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func archiveJSONFixture() archive {
	return archive{1, "mock_cohort_iam_archive", "archive-json", strings.Repeat("a", 40), strings.Repeat("b", 64),
		identity{"iam", "uuid", 40, false, strings.Repeat("c", 64)},
		[]tableImage{{"users", "CREATE TABLE users (id BIGINT CHECK (id > 0))", []column{{"id", "bigint", false}, {"value", "blob", false}},
			[]row{{[]cell{{false, []byte("101")}, {true, []byte{}}}}, {[]cell{{false, []byte("102")}, {false, []byte{}}}}, {[]cell{{false, []byte("103")}, {false, []byte{0, 255, 1}}}}}}}, []tableImage{}}
}

func TestStrictArchiveJSONPreservesLiteralAdmission(t *testing.T) {
	a := archiveJSONFixture()
	raw := encode(a)
	var indented bytes.Buffer
	if err := json.Indent(&indented, raw, "", "  "); err != nil {
		t.Fatal(err)
	}
	replace := func(old, new string) []byte {
		t.Helper()
		if !bytes.Contains(raw, []byte(old)) {
			t.Fatalf("fixture lacks %q", old)
		}
		return bytes.Replace(raw, []byte(old), []byte(new), 1)
	}
	for _, tc := range []struct {
		name  string
		raw   []byte
		valid bool
	}{
		{"canonical_encode", raw, true},
		{"outer_whitespace", append([]byte(" \t\r\n"), raw...), true},
		{"noncanonical_whitespace", indented.Bytes(), true},
		{"noncanonical_key_order", replace(`"format_version":1,"kind":"mock_cohort_iam_archive"`, `"kind":"mock_cohort_iam_archive","format_version":1`), true},
		{"duplicate_same_value", replace(`"format_version":1`, `"format_version":1,"format_version":1`), false},
		{"nested_duplicate", replace(`"dirty":false`, `"dirty":false,"dirty":false`), false},
		{"case_alias", replace(`"format_version":1`, `"Format_Version":1`), false},
		{"null_bool", replace(`"dirty":false`, `"dirty":null`), false},
		{"null_string", replace(`"kind":"mock_cohort_iam_archive"`, `"kind":null`), false},
		{"missing_field", replace(`"format_version":1,`, ``), false},
		{"nested_missing_field", replace(`,"generated":false`, ``), false},
		{"number_alias", replace(`"format_version":1`, `"format_version":1.0`), false},
		{"negative_zero", replace(`"format_version":1`, `"format_version":-0`), false},
		{"byte_array_alias", replace(`"raw":"MTAx"`, `"raw":[49,48,49]`), false},
		{"unknown_field", replace(`"format_version":1`, `"extra":0,"format_version":1`), false},
		{"trailing_document", append(append([]byte{}, raw...), []byte(`{}`)...), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got archive
			err := strictJSON(tc.raw, &got)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
			if tc.valid && !reflect.DeepEqual(got, a) {
				t.Fatal("archive values changed")
			}
		})
	}
	// Nil slices already have literal null semantics; do not tighten them here.
	a.Protected = nil
	var got archive
	if err := strictJSON(encode(a), &got); err != nil || !reflect.DeepEqual(got, a) {
		t.Fatalf("literal nil slice changed: %v", err)
	}
}

func TestStrictArchiveFastPathDoesNotApplyToOpaqueJSON(t *testing.T) {
	var raw json.RawMessage
	if err := strictJSON([]byte(`{"same":1,"same":1}`), &raw); err == nil {
		t.Fatal("opaque duplicate admitted")
	}
}
