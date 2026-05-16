package access

import (
	"context"
	"testing"

	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
	authn "github.com/FangcunMount/iam/v2/internal/pkg/middleware/authn"
	"github.com/FangcunMount/iam/v2/pkg/tenant"
)

type stubRouteAuth struct {
	platformRoles []string
	tenantRoles   map[string][]string
	allowMobile   bool
	err           error
}

func (s stubRouteAuth) AuthorizeRoute(context.Context, string, string, string, string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.allowMobile, nil
}

func (s stubRouteAuth) DirectRoleKeys(_ context.Context, _ string, dom string) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	if dom == tenant.PlatformID {
		return append([]string(nil), s.platformRoles...), nil
	}
	if s.tenantRoles != nil {
		return append([]string(nil), s.tenantRoles[dom]...), nil
	}
	return nil, nil
}

var (
	_ authn.RouteAuthorizationRuntime         = stubRouteAuth{}
	_ appsuggest.ProfileVisibilityIDsResolver = visibilityStub{}
)

func TestOperatingProfileAccessScope_routeAuthNil(t *testing.T) {
	p := NewOperatingProfileAccessScopeProvider(nil, nil)
	scope, err := p.ResolveProfileAccessScope(context.Background(), domainsuggest.OperatingPrincipal{
		OperatorID:   100,
		TenantID:     1,
		TenantDomain: "fangcun",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(scope.TenantIDs) != 0 {
		t.Fatalf("TenantIDs = %v, want empty", scope.TenantIDs)
	}
	if scope.OperatorID != 100 || scope.AllowMobileSearch {
		t.Fatalf("scope = %#v", scope)
	}
}

func TestOperatingProfileAccessScope_platformAdmin(t *testing.T) {
	p := NewOperatingProfileAccessScopeProvider(stubRouteAuth{
		platformRoles: []string{"role:iam:admin"},
	}, nil)
	scope, err := p.ResolveProfileAccessScope(context.Background(), domainsuggest.OperatingPrincipal{
		OperatorID:   100,
		TenantID:     1,
		TenantDomain: "fangcun",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !scope.AllProfile || !scope.AllowMobileSearch {
		t.Fatalf("scope = %#v", scope)
	}
}

func TestOperatingProfileAccessScope_tenantAdminGetsTenantIDs(t *testing.T) {
	p := NewOperatingProfileAccessScopeProvider(stubRouteAuth{
		tenantRoles: map[string][]string{"fangcun": {"role:tenant_admin"}},
		allowMobile: true,
	}, nil)
	scope, err := p.ResolveProfileAccessScope(context.Background(), domainsuggest.OperatingPrincipal{
		OperatorID:   100,
		TenantID:     1,
		TenantDomain: "fangcun",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(scope.TenantIDs) != 1 || scope.TenantIDs[0] != 1 {
		t.Fatalf("TenantIDs = %v", scope.TenantIDs)
	}
	if !scope.AllowMobileSearch {
		t.Fatal("AllowMobileSearch false")
	}
}

func TestOperatingProfileAccessScope_plainUserNoTenantIDs(t *testing.T) {
	p := NewOperatingProfileAccessScopeProvider(stubRouteAuth{
		tenantRoles: map[string][]string{"fangcun": {"role:user"}},
	}, nil)
	scope, err := p.ResolveProfileAccessScope(context.Background(), domainsuggest.OperatingPrincipal{
		OperatorID:   100,
		TenantID:     1,
		TenantDomain: "fangcun",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(scope.TenantIDs) != 0 {
		t.Fatalf("TenantIDs = %v, want empty for plain user", scope.TenantIDs)
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
		routeAuth      authn.RouteAuthorizationRuntime
		principal      domainsuggest.OperatingPrincipal
		visibility     appsuggest.ProfileVisibilityIDsResolver
		wantAll        bool
		wantTenantIDs  int
		wantOperatorID int64
		wantProfileIDs int
		wantMobile     bool
	}{
		{
			name:           "route_auth_nil",
			principal:      domainsuggest.OperatingPrincipal{OperatorID: 100, TenantID: 1, TenantDomain: "fangcun"},
			wantOperatorID: 100,
		},
		{
			name:       "platform_admin",
			routeAuth:  stubRouteAuth{platformRoles: []string{"role:iam:admin"}},
			principal:  domainsuggest.OperatingPrincipal{OperatorID: 100, TenantID: 1, TenantDomain: "fangcun"},
			wantAll:    true,
			wantMobile: true,
		},
		{
			name:           "tenant_admin",
			routeAuth:      stubRouteAuth{tenantRoles: map[string][]string{"fangcun": {"role:tenant_admin"}}, allowMobile: true},
			principal:      domainsuggest.OperatingPrincipal{OperatorID: 100, TenantID: 1, TenantDomain: "fangcun"},
			wantTenantIDs:  1,
			wantOperatorID: 100,
			wantMobile:     true,
		},
		{
			name:           "plain_user",
			routeAuth:      stubRouteAuth{tenantRoles: map[string][]string{"fangcun": {"role:user"}}},
			principal:      domainsuggest.OperatingPrincipal{OperatorID: 100, TenantID: 1, TenantDomain: "fangcun"},
			wantOperatorID: 100,
		},
		{
			name:           "plain_user_with_visibility",
			routeAuth:      stubRouteAuth{tenantRoles: map[string][]string{"fangcun": {"role:user"}}},
			principal:      domainsuggest.OperatingPrincipal{OperatorID: 100, TenantID: 1, TenantDomain: "fangcun"},
			visibility:     visibilityStub{ids: []int64{7, 9}},
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
			if len(scope.TenantIDs) != tc.wantTenantIDs {
				t.Fatalf("TenantIDs = %v, want len %d", scope.TenantIDs, tc.wantTenantIDs)
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
	p := NewOperatingProfileAccessScopeProvider(stubRouteAuth{
		tenantRoles: map[string][]string{"fangcun": {"role:user"}},
	}, visibilityStub{ids: []int64{7, 9, 7}})
	scope, err := p.ResolveProfileAccessScope(context.Background(), domainsuggest.OperatingPrincipal{
		OperatorID:   100,
		TenantID:     1,
		TenantDomain: "fangcun",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(scope.ProfileIDs) != 2 || scope.ProfileIDs[0] != 7 || scope.ProfileIDs[1] != 9 {
		t.Fatalf("ProfileIDs = %v", scope.ProfileIDs)
	}
}
