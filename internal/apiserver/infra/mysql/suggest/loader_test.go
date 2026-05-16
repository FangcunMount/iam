package suggest

import "testing"

func TestRecordMapsToProfileSearchTerm(t *testing.T) {
	mobiles := " 13800138000, ,13900139000 "
	owners := "100,200"
	row := record{
		ID:               7,
		Name:             " 张三 ",
		TenantID:         1,
		OrgID:            2,
		Mobiles:          &mobiles,
		OwnerOperatorIDs: &owners,
		Weight:           5,
	}

	term := row.profileSearchTerm()

	if term.ProfileID != 7 || term.DisplayName != "张三" || term.Weight != 5 || term.TenantID != 1 || term.OrgID != 2 {
		t.Fatalf("term = %#v", term)
	}
	if len(term.Mobiles) != 2 || term.Mobiles[0] != "13800138000" || term.Mobiles[1] != "13900139000" {
		t.Fatalf("mobiles = %#v", term.Mobiles)
	}
	if len(term.OwnerOperatorIDs) != 2 || term.OwnerOperatorIDs[0] != 100 || term.OwnerOperatorIDs[1] != 200 {
		t.Fatalf("OwnerOperatorIDs = %#v", term.OwnerOperatorIDs)
	}
}

func TestRecordOwnerOperatorFromCreatedByCSV(t *testing.T) {
	owners := "42"
	row := record{ID: 1, Name: "a", OwnerOperatorIDs: &owners}
	term := row.profileSearchTerm()
	if len(term.OwnerOperatorIDs) != 1 || term.OwnerOperatorIDs[0] != 42 {
		t.Fatalf("OwnerOperatorIDs = %#v", term.OwnerOperatorIDs)
	}
}
