package visibility_test

import (
	"testing"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
)

func TestScopeAllowsDimensions(t *testing.T) {
	scope := visibility.NewScope(false, true, 100, []int64{10}, []int64{1, 2})

	if !scope.Allows(visibility.Resource{ProfileID: 1}) {
		t.Fatal("profile id should allow")
	}
	if !scope.Allows(visibility.Resource{OrgID: 10}) {
		t.Fatal("org id should allow")
	}
	if !scope.Allows(visibility.Resource{OwnerOperatorIDs: []int64{100}}) {
		t.Fatal("operator id should allow")
	}
	if scope.Allows(visibility.Resource{ProfileID: 99, OrgID: 99}) {
		t.Fatal("should deny unknown resource")
	}
}

func TestResolutionPolicyPlatformAllProfiles(t *testing.T) {
	policy := visibility.ResolutionPolicy{}
	scope := policy.Resolve(
		visibility.Principal{OperatorID: 1, TenantDomain: "fangcun"},
		visibility.AuthorizationFacts{PlatformListAllowed: true, PlatformMobileSearchAllowed: true},
		nil,
	)
	if !scope.IsAllProfiles() || !scope.AllowsMobileSearch() {
		t.Fatalf("scope = %+v", scope)
	}
}

func TestResolutionPolicyTenantMobileOnlyFromFacts(t *testing.T) {
	policy := visibility.ResolutionPolicy{}
	scope := policy.Resolve(
		visibility.Principal{OperatorID: 1, TenantDomain: "fangcun", OrgID: 5},
		visibility.AuthorizationFacts{TenantMobileSearchAllowed: true},
		[]int64{7},
	)
	if scope.IsAllProfiles() || !scope.AllowsMobileSearch() {
		t.Fatal("tenant scope mismatch")
	}
	if !scope.Allows(visibility.Resource{ProfileID: 7}) {
		t.Fatal("visibility profile should allow")
	}
}
