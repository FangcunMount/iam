package suggest

import "testing"

func TestScopeResolutionPolicyPlatformList(t *testing.T) {
	p := ScopeResolutionPolicy{}
	scope := p.Resolve(
		OperatingPrincipal{OperatorID: 100, OrgID: 1},
		ProfileAuthorizationFacts{PlatformListAllowed: true, PlatformMobileSearchAllowed: true},
		[]int64{99},
	)
	if !scope.AllProfile || !scope.AllowMobileSearch || len(scope.ProfileIDs) != 0 {
		t.Fatalf("scope = %#v", scope)
	}
}

func TestScopeResolutionPolicyTenantMobileWithVisibility(t *testing.T) {
	p := ScopeResolutionPolicy{}
	scope := p.Resolve(
		OperatingPrincipal{OperatorID: 100, OrgID: 1, TenantDomain: "fangcun"},
		ProfileAuthorizationFacts{TenantMobileSearchAllowed: true},
		[]int64{7, 9, 7},
	)
	if scope.AllProfile || scope.OperatorID != 100 || !scope.AllowMobileSearch {
		t.Fatalf("scope = %#v", scope)
	}
	if len(scope.ProfileIDs) != 2 || scope.ProfileIDs[0] != 7 || scope.ProfileIDs[1] != 9 {
		t.Fatalf("ProfileIDs = %v", scope.ProfileIDs)
	}
}

func TestScopeResolutionPolicyNilFactsRestrictedScope(t *testing.T) {
	p := ScopeResolutionPolicy{}
	scope := p.Resolve(
		OperatingPrincipal{OperatorID: 100, OrgID: 1},
		ProfileAuthorizationFacts{},
		nil,
	)
	if scope.AllProfile || scope.AllowMobileSearch || scope.OperatorID != 100 {
		t.Fatalf("scope = %#v", scope)
	}
}
