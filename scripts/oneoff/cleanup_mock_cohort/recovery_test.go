package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func tinyImage(name string, rs ...[]string) tableImage {
	im := tableImage{Name: name, DDL: "fixed schema", Columns: []column{{Name: "id", Type: "bigint unsigned"}, {Name: "value", Type: "varbinary(128)"}}, Rows: []row{}}
	for _, vals := range rs {
		r := row{Values: []cell{}}
		for _, v := range vals {
			r.Values = append(r.Values, cell{Raw: []byte(v)})
		}
		im.Rows = append(im.Rows, r)
	}
	return im
}
func TestUnknownCommitReadbackDistinguishesAtomicStates(t *testing.T) {
	before := tinyImage("users", []string{"101", "active"}, []string{"102", "active"})
	after := tinyImage("users", []string{"101", "inactive"}, []string{"102", "inactive"})
	for _, tc := range []struct {
		name    string
		current []row
		want    string
	}{{"lost-response-after-commit", after.Rows, "readback_after"}, {"rollback", before.Rows, "readback_before"}, {"foreign-or-partial", []row{before.Rows[0], after.Rows[1]}, "readback_mixed"}, {"deleted", []row{}, "readback_mixed"}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := reconcileRows(tc.current, before.Rows, after.Rows); got != tc.want {
				t.Fatalf("%s", got)
			}
		})
	}
}
func TestJournalPrecommitBytesSurviveUnknownAndCannotBeRebound(t *testing.T) {
	d := t.TempDir()
	_ = os.Chmod(d, 0700)
	path := filepath.Join(d, "journal.jsonl")
	sha := strings.Repeat("a", 64)
	j, e := newJournal(path, "op", sha)
	if e != nil {
		t.Fatal(e)
	}
	before := tinyImage("users", []string{"101", "active"})
	after := tinyImage("users", []string{"101", "inactive"})
	if e = j.append("deactivate_users", before, after, "prepared"); e != nil {
		t.Fatal(e)
	}
	original, _ := os.ReadFile(path)
	if e = j.append("deactivate_users", before, after, "readback_after"); e != nil {
		t.Fatal(e)
	}
	_ = j.f.Close()
	raw, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(raw), string(original)) {
		t.Fatal("rewrote unknown precommit original")
	}
	entries, e := parseJournal(raw, "op", sha)
	if e != nil || len(entries) != 2 {
		t.Fatal(e)
	}
	if got := knownPostimages(entries)["users:101"]; len(got) != 1 || !reflect.DeepEqual(got[0], after.Rows[0]) {
		t.Fatal("missing exact own postimage")
	}
	for _, bad := range [][]byte{append([]byte{}, raw[:len(raw)-1]...), []byte(strings.Replace(string(raw), `"sequence":2`, `"sequence":3`, 1)), []byte(strings.Replace(string(raw), `"value"`, `"foreign"`, 1))} {
		if _, e = parseJournal(bad, "op", sha); e == nil {
			t.Fatal("tampered or partial journal admitted")
		}
	}
	if _, e = parseJournal(raw, "other", sha); e == nil {
		t.Fatal("foreign operation admitted")
	}
	if _, e = newJournal(path, "op", sha); e == nil {
		t.Fatal("journal overwritten")
	}
}
func TestSharedRelationshipsBlockWholeCohort(t *testing.T) {
	s := iamScope{[]string{"101"}, []string{"201"}, []string{"301"}}
	users := tinyImage("users", []string{"101", "name"})
	profiles := tinyImage("profiles", []string{"201", "name"})
	login := tinyImage("auth_login_identities", []string{"301", "101"})
	login.Columns[1].Name = "user_id"
	link := tableImage{Name: "profile_links", Columns: []column{{Name: "id"}, {Name: "user_id"}, {Name: "profile_id"}}, Rows: []row{{Values: []cell{{Raw: []byte("401")}, {Raw: []byte("101")}, {Raw: []byte("201")}}}}}
	images := []tableImage{users, profiles, login, link}
	if e := validateClosure(images, s); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name   string
		change func([]tableImage)
	}{{"shared-profile", func(a []tableImage) { a[3].Rows[0].Values[1].Raw = []byte("999") }}, {"unselected-profile", func(a []tableImage) { a[3].Rows[0].Values[2].Raw = []byte("999") }}, {"retained-login-owner", func(a []tableImage) { a[2].Rows[0].Values[1].Raw = []byte("999") }}} {
		t.Run(tc.name, func(t *testing.T) {
			var copyImages []tableImage
			if e := strictJSON(mustMarshal(images), &copyImages); e != nil {
				t.Fatal(e)
			}
			tc.change(copyImages)
			if e := validateClosure(copyImages, s); e == nil {
				t.Fatal("shared data admitted")
			}
		})
	}
}
func TestScopeReadBatchesStayBoundedAndTyped(t *testing.T) {
	s := iamScope{Users: []string{}, Profiles: []string{}, LoginIdentities: []string{}}
	for i := 1; i <= 2501; i++ {
		s.Users = append(s.Users, strings.Repeat("1", i%5+1))
	}
	s.Profiles = []string{"1"}
	b := scopeBatches(s)
	if len(b) != 3 || len(b[0].Users) != 1000 || len(b[2].Users) != 501 || len(b[1].Profiles) != 0 {
		t.Fatal("read chunks not bounded")
	}
	for _, batch := range b {
		_, args := scopePredicate("profile_links", batch)
		if len(args) > 2000 {
			t.Fatal("oversized SQL bind")
		}
	}
}
func TestNullAndBinaryArchiveCellsStayDistinct(t *testing.T) {
	values := []cell{{Null: true, Raw: []byte{}}, {Null: false, Raw: []byte{}}, {Null: false, Raw: []byte{0, 255, 1}}}
	var got []cell
	if e := strictJSON(mustMarshal(values), &got); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(values, got) {
		t.Fatal("NULL/empty/binary changed")
	}
}
func TestDefaultGeneratedTimestampIsRestoredAsAnOriginalValue(t *testing.T) {
	for _, extra := range []string{"", "DEFAULT_GENERATED", "DEFAULT_GENERATED on update CURRENT_TIMESTAMP"} {
		if generatedColumn(extra) {
			t.Fatal("default timestamp mistaken for an unwriteable generated column")
		}
	}
	for _, extra := range []string{"VIRTUAL GENERATED", "STORED GENERATED"} {
		if !generatedColumn(extra) {
			t.Fatal("computed column would be inserted")
		}
	}
}
