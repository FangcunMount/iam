package access

import (
	"context"
	"fmt"
	"strconv"

	authorizationapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	appsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest"
	domainsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest"
	"github.com/FangcunMount/iam/v3/pkg/tenant"
)

// ProfileAuthorizationFactsReader 从 AuthZ 查询 Suggest 授权事实。
type ProfileAuthorizationFactsReader struct {
	permissions authorizationapp.RoutePermissionChecker
}

// NewProfileAuthorizationFactsReader 创建 reader；permissions 为 nil 时返回零事实。
func NewProfileAuthorizationFactsReader(permissions authorizationapp.RoutePermissionChecker) *ProfileAuthorizationFactsReader {
	return &ProfileAuthorizationFactsReader{permissions: permissions}
}

// ResolveAuthorizationFacts 实现 application 端口。
func (r *ProfileAuthorizationFactsReader) ResolveAuthorizationFacts(
	ctx context.Context,
	principal domainsuggest.OperatingPrincipal,
) (domainsuggest.ProfileAuthorizationFacts, error) {
	if r == nil || r.permissions == nil {
		return domainsuggest.ProfileAuthorizationFacts{}, nil
	}
	sub := "user:" + strconv.FormatInt(principal.OperatorID, 10)

	listAllowed, err := r.permissions.CheckRoutePermission(
		ctx, sub, tenant.PlatformID, appsuggest.ResourceIAMProfileCollection, appsuggest.ActionList,
	)
	if err != nil {
		return domainsuggest.ProfileAuthorizationFacts{}, err
	}
	if listAllowed {
		mobileAllowed, err := r.permissions.CheckRoutePermission(
			ctx, sub, tenant.PlatformID, appsuggest.ResourceIAMProfileCollection, appsuggest.ActionSearchByMobile,
		)
		if err != nil {
			return domainsuggest.ProfileAuthorizationFacts{}, err
		}
		return domainsuggest.ProfileAuthorizationFacts{
			PlatformListAllowed:         true,
			PlatformMobileSearchAllowed: mobileAllowed,
		}, nil
	}

	tenantDom := principal.TenantDomain
	if tenantDom == "" {
		tenantDom = tenant.DefaultID
	}
	mobileOK, err := r.permissions.CheckRoutePermission(
		ctx, sub, tenantDom, appsuggest.ResourceIAMProfileCollection, appsuggest.ActionSearchByMobile,
	)
	if err != nil {
		return domainsuggest.ProfileAuthorizationFacts{}, err
	}
	return domainsuggest.ProfileAuthorizationFacts{
		TenantMobileSearchAllowed: mobileOK,
	}, nil
}

var _ appsuggest.ProfileAuthorizationFactsReader = (*ProfileAuthorizationFactsReader)(nil)

// OperatingProfileAccessScopeProvider 保留为 resolver 的 thin wrapper，供 container 过渡装配。
type OperatingProfileAccessScopeProvider struct {
	resolver *appsuggest.ProfileAccessScopeResolver
}

// NewOperatingProfileAccessScopeProvider 创建 scope provider。
func NewOperatingProfileAccessScopeProvider(
	permissions authorizationapp.RoutePermissionChecker,
	visibility appsuggest.ProfileVisibilityIDsResolver,
) *OperatingProfileAccessScopeProvider {
	factsReader := NewProfileAuthorizationFactsReader(permissions)
	return &OperatingProfileAccessScopeProvider{
		resolver: appsuggest.NewProfileAccessScopeResolver(factsReader, visibility),
	}
}

// ResolveProfileAccessScope 实现 ProfileAccessScopeProvider。
func (p *OperatingProfileAccessScopeProvider) ResolveProfileAccessScope(
	ctx context.Context,
	principal domainsuggest.OperatingPrincipal,
) (domainsuggest.ProfileAccessScope, error) {
	if p == nil || p.resolver == nil {
		return domainsuggest.ProfileAccessScope{}, fmt.Errorf("profile access scope provider is nil")
	}
	return p.resolver.ResolveProfileAccessScope(ctx, principal)
}

var _ appsuggest.ProfileAccessScopeProvider = (*OperatingProfileAccessScopeProvider)(nil)
