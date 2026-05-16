package access

import (
	"context"
	"testing"

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

var _ authn.RouteAuthorizationRuntime = stubRouteAuth{}

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
