package suggest

import (
	"strings"
	"testing"
)

func TestDefaultSQLUsesSameActiveRelationshipEligibility(t *testing.T) {
	loader := NewLoader(nil, LoaderConfig{})
	full := loader.config.FullSQL
	delta := loader.config.DeltaSQL

	for _, sql := range []string{full, delta} {
		if !strings.Contains(sql, "g.deleted_at IS NULL AND g.revoked_at IS NULL") {
			t.Fatalf("query does not require active profile link: %s", sql)
		}
		if !strings.Contains(sql, "u.deleted_at IS NULL") {
			t.Fatalf("query does not require active user: %s", sql)
		}
	}
	for _, fragment := range []string{
		"FROM profile_links g",
		"g.revoked_at > ?",
		"u.deleted_at > ?",
		"NOT EXISTS",
		"'' AS name",
	} {
		if !strings.Contains(delta, fragment) {
			t.Fatalf("delta query missing %q", fragment)
		}
	}
}

func TestRecordMapsToSuggestibleProfile(t *testing.T) {
	mobiles := " 13800138000, ,13900139000 "
	owners := "100,200"
	row := record{
		ID:               7,
		Name:             " 张三 ",
		OrgID:            2,
		Mobiles:          &mobiles,
		OwnerOperatorIDs: &owners,
		Weight:           5,
	}

	p, err := row.fullProfile()
	if err != nil {
		t.Fatal(err)
	}

	if p.ID() != 7 || p.DisplayName() != "张三" || p.Weight() != 5 || p.OrgID() != 2 {
		t.Fatalf("profile = %#v", p)
	}
	mobileList := p.Mobiles()
	if len(mobileList) != 2 || mobileList[0] != "13800138000" || mobileList[1] != "13900139000" {
		t.Fatalf("mobiles = %#v", mobileList)
	}
	ownerList := p.OwnerOperatorIDs()
	if len(ownerList) != 2 || ownerList[0] != 100 || ownerList[1] != 200 {
		t.Fatalf("OwnerOperatorIDs = %#v", ownerList)
	}
}

func TestRecordOwnerOperatorFromCreatedByCSV(t *testing.T) {
	owners := "42"
	row := record{ID: 1, Name: "a", OwnerOperatorIDs: &owners}
	p, err := row.fullProfile()
	if err != nil {
		t.Fatal(err)
	}
	ownerList := p.OwnerOperatorIDs()
	if len(ownerList) != 1 || ownerList[0] != 42 {
		t.Fatalf("OwnerOperatorIDs = %#v", ownerList)
	}
}

func TestRecordFullProfileRejectsMalformedProjection(t *testing.T) {
	for _, row := range []record{
		{ID: 0, Name: "a"},
		{ID: 1, Name: "  "},
	} {
		if _, err := row.fullProfile(); err == nil {
			t.Fatalf("fullProfile(%+v) error = nil", row)
		}
	}
}

func TestRecordDeltaChangeRejectsInvalidID(t *testing.T) {
	if _, err := (record{ID: 0, Name: "a"}).deltaChange(); err == nil {
		t.Fatal("deltaChange() error = nil")
	}
}
