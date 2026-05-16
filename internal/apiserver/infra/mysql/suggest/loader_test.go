package suggest

import "testing"

func TestRecordMapsToProfileSearchTerm(t *testing.T) {
	mobiles := " 13800138000, ,13900139000 "
	row := record{
		ID:               7,
		Name:             " 张三 ",
		TenantID:         1,
		OrgID:            2,
		Mobiles:          &mobiles,
		OwnerOperatorIDs: nil,
		Weight:           5,
	}

	term := row.profileSearchTerm()

	if term.ProfileID != 7 || term.DisplayName != "张三" || term.Weight != 5 || term.TenantID != 1 || term.OrgID != 2 {
		t.Fatalf("term = %#v", term)
	}
	if len(term.Mobiles) != 2 || term.Mobiles[0] != "13800138000" || term.Mobiles[1] != "13900139000" {
		t.Fatalf("mobiles = %#v", term.Mobiles)
	}
}
