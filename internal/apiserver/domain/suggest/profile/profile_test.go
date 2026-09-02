package profile_test

import (
	"testing"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/profile"
)

func TestSuggestibleProfileNormalizes(t *testing.T) {
	p := profile.New(42, " 张三 ", []string{" 13800138000 ", "", "13900139000"}, 7, 0, []int64{99, 99, 0})

	if p.DisplayName() != "张三" {
		t.Fatalf("DisplayName = %q, want 张三", p.DisplayName())
	}
	mobiles := p.Mobiles()
	if len(mobiles) != 2 || mobiles[0] != "13800138000" || mobiles[1] != "13900139000" {
		t.Fatalf("Mobiles = %#v", mobiles)
	}
	if p.ID() != 42 || p.Weight() != 7 || p.OrgID() != 0 {
		t.Fatalf("fields = (%d,%d,%d)", p.ID(), p.Weight(), p.OrgID())
	}
	owners := p.OwnerOperatorIDs()
	if len(owners) != 1 || owners[0] != 99 {
		t.Fatalf("OwnerOperatorIDs = %#v", owners)
	}
}

func TestMobileDisclosurePolicy(t *testing.T) {
	policy := profile.MobileDisclosurePolicy{}
	if got := policy.Disclose([]string{"13800138000"}); got != "138****8000" {
		t.Fatalf("mask = %q", got)
	}
	policy.DisableMask = true
	if got := policy.Disclose([]string{"13800138000"}); got != "13800138000" {
		t.Fatalf("plain = %q", got)
	}
}
