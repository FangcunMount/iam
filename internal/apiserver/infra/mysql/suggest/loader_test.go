package suggest

import "testing"

func TestRecordMapsToProfileCandidate(t *testing.T) {
	mobiles := " 13800138000, ,13900139000 "
	row := record{
		ID:      7,
		Name:    " 张三 ",
		Mobiles: &mobiles,
		Weight:  5,
	}

	candidate := row.profileCandidate()

	if candidate.ProfileID != 7 || candidate.DisplayName != "张三" || candidate.Weight != 5 {
		t.Fatalf("candidate = %#v", candidate)
	}
	if len(candidate.Mobiles) != 2 || candidate.Mobiles[0] != "13800138000" || candidate.Mobiles[1] != "13900139000" {
		t.Fatalf("mobiles = %#v", candidate.Mobiles)
	}
}
