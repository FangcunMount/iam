package suggest

import (
	"context"
	"fmt"
	"strconv"

	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
	authn "github.com/FangcunMount/iam/v2/internal/pkg/middleware/authn"
	"github.com/FangcunMount/iam/v2/pkg/tenant"
)

// OperatingProfileAccessScopeProvider 将 operating 身份解析为 ProfileAccessScope。
type OperatingProfileAccessScopeProvider struct {
	routeAuth authn.RouteAuthorizationRuntime
}

// NewOperatingProfileAccessScopeProvider 创建 Provider；routeAuth 可为 nil（降级为保守范围）。
func NewOperatingProfileAccessScopeProvider(routeAuth authn.RouteAuthorizationRuntime) *OperatingProfileAccessScopeProvider {
	return &OperatingProfileAccessScopeProvider{routeAuth: routeAuth}
}

// ResolveProfileAccessScope 实现 ProfileAccessScopeProvider。
func (p *OperatingProfileAccessScopeProvider) ResolveProfileAccessScope(
	ctx context.Context,
	principal domainsuggest.OperatingPrincipal,
) (domainsuggest.ProfileAccessScope, error) {
	if p == nil {
		return domainsuggest.ProfileAccessScope{}, fmt.Errorf("profile access scope provider is nil")
	}
	if principal.IsSuperAdmin {
		return domainsuggest.ProfileAccessScope{
			AllProfile:        true,
			AllowMobileSearch: true,
		}, nil
	}

	sub := "user:" + strconv.FormatInt(principal.OperatorID, 10)
	if p.routeAuth == nil {
		return domainsuggest.ProfileAccessScope{
			TenantIDs:         tenantIDsForPrincipal(principal),
			OrgIDs:            append([]int64(nil), principal.OrgIDs...),
			OperatorID:        principal.OperatorID,
			AllowMobileSearch: false,
		}, nil
	}

	domains := []string{principal.TenantDomain}
	if principal.TenantDomain != "" && principal.TenantDomain != tenant.PlatformID {
		domains = append(domains, tenant.PlatformID)
	}

	for _, dom := range domains {
		if dom == "" {
			continue
		}
		roles, err := p.routeAuth.DirectRoleKeys(ctx, sub, dom)
		if err != nil {
			return domainsuggest.ProfileAccessScope{}, err
		}
		for _, r := range roles {
			if authn.IsPlatformAdminRole(r) {
				return domainsuggest.ProfileAccessScope{
					AllProfile:        true,
					AllowMobileSearch: true,
				}, nil
			}
		}
	}

	mobileOK := false
	domain := principal.TenantDomain
	if domain == "" {
		domain = tenant.DefaultID
	}
	allowed, err := p.routeAuth.AuthorizeRoute(ctx, sub, domain, ResourceIAMProfileCollection, ActionSearchByMobile)
	if err == nil && allowed {
		mobileOK = true
	}

	return domainsuggest.ProfileAccessScope{
		TenantIDs:         tenantIDsForPrincipal(principal),
		OrgIDs:            append([]int64(nil), principal.OrgIDs...),
		OperatorID:        principal.OperatorID,
		AllowMobileSearch: mobileOK,
	}, nil
}

func tenantIDsForPrincipal(p domainsuggest.OperatingPrincipal) []int64 {
	if p.TenantID > 0 {
		return []int64{p.TenantID}
	}
	return nil
}
