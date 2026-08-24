package cache

import "testing"

func TestFamiliesReturnsCatalogSnapshot(t *testing.T) {
	families := Families()
	if len(families) != 14 {
		t.Fatalf("family count = %d, want 14", len(families))
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
	if quota.KeyPattern != "otp:quota:{scene}:{phoneE164}:{dimension}" {
		t.Fatalf("quota KeyPattern = %q", quota.KeyPattern)
	}
	if quota.RedisType != RedisDataTypeZSet {
		t.Fatalf("quota RedisType = %q, want %q", quota.RedisType, RedisDataTypeZSet)
	}

	descriptor, ok := GetFamily(FamilyIDPWechatAccessToken)
	if !ok {
		t.Fatalf("GetFamily(%s) ok = false, want true", FamilyIDPWechatAccessToken)
	}
	if descriptor.OwnerModule != "idp" {
		t.Fatalf("OwnerModule = %q, want idp", descriptor.OwnerModule)
	}

	suggestRedis, ok := GetFamily(FamilySuggestRedisRateLimit)
	if !ok || suggestRedis.OwnerModule != "suggest" || suggestRedis.Backend != BackendKindRedis {
		t.Fatalf("suggest Redis descriptor = %#v, ok=%v", suggestRedis, ok)
	}

	_, ok = GetFamily("unknown.family")
	if ok {
		t.Fatalf("GetFamily(unknown) ok = true, want false")
	}
}
