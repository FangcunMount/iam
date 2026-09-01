package access

import (
	"context"
	"testing"

	authorizationapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	appsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest"
	domainsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest"
	"github.com/FangcunMount/iam/v3/pkg/tenant"
)

type stubRouteAuth struct {
	allowPlatformProfiles bool
	allowPlatformMobile   bool
	allowTenantMobile     bool
	err                   error
}

func (s stubRouteAuth) CheckRoutePermission(_ context.Context, _, domain, _, action string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	if domain == tenant.PlatformID {
		if action == appsuggest.ActionList {
			return s.allowPlatformProfiles, nil
		}
		if action == appsuggest.ActionSearchByMobile {
			return s.allowPlatformMobile, nil
		}
	}
	return action == appsuggest.ActionSearchByMobile && s.allowTenantMobile, nil
}

var (
	_ authorizationapp.RoutePermissionChecker = stubRouteAuth{}
	_ appsuggest.ProfileVisibilityIDsResolver = visibilityStub{}
)

func TestOperatingProfileAccessScope_routeAuthNil(t *testing.T) {
	p := NewOperatingProfileAccessScopeProvider(nil, nil)
	scope, err := p.ResolveProfileAccessScope(context.Background(), domainsuggest.OperatingPrincipal{
		OperatorID:   100,
		TenantDomain: "fangcun",
	})
	if err != nil {
		t.Fatal(err)
	}
	if scope.OperatorID != 100 || scope.AllowMobileSearch {
		t.Fatalf("scope = %#v", scope)
	}
}

func TestOperatingProfileAccessScope_platformProfilePermission(t *testing.T) {
	p := NewOperatingProfileAccessScopeProvider(stubRouteAuth{
		allowPlatformProfiles: true,
		allowPlatformMobile:   true,
	}, nil)
	scope, err := p.ResolveProfileAccessScope(context.Background(), domainsuggest.OperatingPrincipal{
		OperatorID:   100,
		TenantDomain: "fangcun",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !scope.AllProfile || !scope.AllowMobileSearch {
		t.Fatalf("scope = %#v", scope)
	}
}

func TestOperatingProfileAccessScope_tenantAdminGetsOrgIDs(t *testing.T) {
	p := NewOperatingProfileAccessScopeProvider(stubRouteAuth{allowTenantMobile: true}, nil)
	scope, err := p.ResolveProfileAccessScope(context.Background(), domainsuggest.OperatingPrincipal{
		OperatorID:   100,
		OrgID:        1,
		TenantDomain: "fangcun",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(scope.OrgIDs) != 1 || scope.OrgIDs[0] != 1 {
		t.Fatalf("OrgIDs = %v", scope.OrgIDs)
	}
	if !scope.AllowMobileSearch {
		t.Fatal("AllowMobileSearch false")
	}
}

func TestOperatingProfileAccessScope_plainUserUsesOperatorScope(t *testing.T) {
	p := NewOperatingProfileAccessScopeProvider(stubRouteAuth{}, nil)
	scope, err := p.ResolveProfileAccessScope(context.Background(), domainsuggest.OperatingPrincipal{
		OperatorID:   100,
		TenantDomain: "fangcun",
	})
	if err != nil {
		t.Fatal(err)
	}
	if scope.OperatorID != 100 {
		t.Fatalf("OperatorID = %d", scope.OperatorID)
	}
}

type visibilityStub struct {
	ids []int64
	err error
}

func (v visibilityStub) VisibleProfileIDs(context.Context, domainsuggest.OperatingPrincipal) ([]int64, error) {
	if v.err != nil {
		return nil, v.err
	}
	return append([]int64(nil), v.ids...), nil
}

func TestOperatingProfileAccessScope_roleMatrix(t *testing.T) {
	cases := []struct {
		name           string
		routeAuth      authorizationapp.RoutePermissionChecker
		principal      domainsuggest.OperatingPrincipal
		visibility     appsuggest.ProfileVisibilityIDsResolver
		wantAll        bool
		wantOrgIDs     int
		wantOperatorID int64
		wantProfileIDs int
		wantMobile     bool
	}{
		{
			name:           "route_auth_nil",
			principal:      domainsuggest.OperatingPrincipal{OperatorID: 100, OrgID: 1, TenantDomain: "fangcun"},
			wantOrgIDs:     1,
			wantOperatorID: 100,
		},
		{
			name:       "platform_profile_permission",
			routeAuth:  stubRouteAuth{allowPlatformProfiles: true, allowPlatformMobile: true},
			principal:  domainsuggest.OperatingPrincipal{OperatorID: 100, OrgID: 1, TenantDomain: "fangcun"},
			wantAll:    true,
			wantMobile: true,
		},
		{
			name:           "tenant_profile_permission",
			routeAuth:      stubRouteAuth{allowTenantMobile: true},
			principal:      domainsuggest.OperatingPrincipal{OperatorID: 100, OrgID: 1, TenantDomain: "fangcun"},
			wantOrgIDs:     1,
			wantOperatorID: 100,
			wantMobile:     true,
		},
		{
			name:           "plain_user",
			routeAuth:      stubRouteAuth{},
			principal:      domainsuggest.OperatingPrincipal{OperatorID: 100, OrgID: 1, TenantDomain: "fangcun"},
			wantOrgIDs:     1,
			wantOperatorID: 100,
		},
		{
			name:           "plain_user_with_visibility",
			routeAuth:      stubRouteAuth{},
			principal:      domainsuggest.OperatingPrincipal{OperatorID: 100, OrgID: 1, TenantDomain: "fangcun"},
			visibility:     visibilityStub{ids: []int64{7, 9}},
			wantOrgIDs:     1,
			wantOperatorID: 100,
			wantProfileIDs: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var vis appsuggest.ProfileVisibilityIDsResolver
			if tc.visibility != nil {
				vis = tc.visibility
			}
			p := NewOperatingProfileAccessScopeProvider(tc.routeAuth, vis)
			scope, err := p.ResolveProfileAccessScope(context.Background(), tc.principal)
			if err != nil {
				t.Fatal(err)
			}
			if scope.AllProfile != tc.wantAll {
				t.Fatalf("AllProfile = %v, want %v", scope.AllProfile, tc.wantAll)
			}
			if len(scope.OrgIDs) != tc.wantOrgIDs {
				t.Fatalf("OrgIDs = %v, want len %d", scope.OrgIDs, tc.wantOrgIDs)
			}
			if scope.OperatorID != tc.wantOperatorID {
				t.Fatalf("OperatorID = %d, want %d", scope.OperatorID, tc.wantOperatorID)
			}
			if len(scope.ProfileIDs) != tc.wantProfileIDs {
				t.Fatalf("ProfileIDs = %v, want len %d", scope.ProfileIDs, tc.wantProfileIDs)
			}
			if scope.AllowMobileSearch != tc.wantMobile {
				t.Fatalf("AllowMobileSearch = %v, want %v", scope.AllowMobileSearch, tc.wantMobile)
			}
		})
	}
}

func TestOperatingProfileAccessScope_visibilityMerge(t *testing.T) {
	p := NewOperatingProfileAccessScopeProvider(stubRouteAuth{}, visibilityStub{ids: []int64{7, 9, 7}})
	scope, err := p.ResolveProfileAccessScope(context.Background(), domainsuggest.OperatingPrincipal{
		OperatorID:   100,
		TenantDomain: "fangcun",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(scope.ProfileIDs) != 2 || scope.ProfileIDs[0] != 7 || scope.ProfileIDs[1] != 9 {
		t.Fatalf("ProfileIDs = %v", scope.ProfileIDs)
	}
}
