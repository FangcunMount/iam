package cache

import "testing"

func TestFamiliesReturnsCatalogSnapshot(t *testing.T) {
	families := Families()
	if len(families) != 11 {
		t.Fatalf("family count = %d, want 11", len(families))
	}
	if families[0].Family != FamilyAuthnRefreshToken {
		t.Fatalf("first family = %s, want %s", families[0].Family, FamilyAuthnRefreshToken)
	}
}

func TestFamiliesReturnsCopy(t *testing.T) {
	families := Families()
	families[0].Family = "changed"
	families[0].Capabilities[0] = "mutated"

	again := Families()
	if again[0].Family != FamilyAuthnRefreshToken {
		t.Fatalf("catalog family was mutated: %s", again[0].Family)
	}
	if again[0].Capabilities[0] != GovernanceCapabilityInspect {
		t.Fatalf("catalog capability was mutated: %s", again[0].Capabilities[0])
	}
}

func TestGetFamily(t *testing.T) {
	quota, ok := GetFamily(FamilyAuthnLoginOTPSendQuota)
	if !ok {
		t.Fatalf("GetFamily(%s) ok = false, want true", FamilyAuthnLoginOTPSendQuota)
	}
	if quota.KeyPattern != "otp:quota:{scene}:{phoneE164}:{dimension}:{bucket}" {
		t.Fatalf("quota KeyPattern = %q", quota.KeyPattern)
	}

	descriptor, ok := GetFamily(FamilyIDPWechatAccessToken)
	if !ok {
		t.Fatalf("GetFamily(%s) ok = false, want true", FamilyIDPWechatAccessToken)
	}
	if descriptor.OwnerModule != "idp" {
		t.Fatalf("OwnerModule = %q, want idp", descriptor.OwnerModule)
	}

	_, ok = GetFamily("unknown.family")
	if ok {
		t.Fatalf("GetFamily(unknown) ok = true, want false")
	}
}
