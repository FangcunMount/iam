package access

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	appsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest"
	domainsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest"
	authn "github.com/FangcunMount/iam/v3/internal/pkg/middleware/authn"
	"github.com/FangcunMount/iam/v3/pkg/tenant"
)

// OperatingProfileAccessScopeProvider 将 operating 身份与可选的 ProfileID 解析器合并为 ProfileAccessScope。
// 实现 appsuggest.ProfileAccessScopeProvider，放在 infra 层以免 application/suggest 依赖 AuthZ 细节。
type OperatingProfileAccessScopeProvider struct {
	routeAuth  authn.RouteAuthorizationRuntime
	visibility appsuggest.ProfileVisibilityIDsResolver
}

// NewOperatingProfileAccessScopeProvider 创建 Provider。
// routeAuth 可为 nil（降级为仅 OperatorID + OrgIDs）；visibility 可为 nil。
func NewOperatingProfileAccessScopeProvider(
	routeAuth authn.RouteAuthorizationRuntime,
	visibility appsuggest.ProfileVisibilityIDsResolver,
) *OperatingProfileAccessScopeProvider {
	return &OperatingProfileAccessScopeProvider{routeAuth: routeAuth, visibility: visibility}
}

// ResolveProfileAccessScope 实现 ProfileAccessScopeProvider。
//
// 权限语义：
// - 平台域具备 profiles/list PermissionGrant：AllProfile；
// - 其余主体：仅 OperatorID + principal.OrgIDs；
// - routeAuth 不可用：仅 OperatorID + OrgIDs。
func (p *OperatingProfileAccessScopeProvider) ResolveProfileAccessScope(
	ctx context.Context,
	principal domainsuggest.OperatingPrincipal,
) (domainsuggest.ProfileAccessScope, error) {
	if p == nil {
		return domainsuggest.ProfileAccessScope{}, fmt.Errorf("profile access scope provider is nil")
	}
	sub := "user:" + strconv.FormatInt(principal.OperatorID, 10)
	if p.routeAuth == nil {
		out := p.scopeOperatorAndOrg(principal, false)
		if err := p.mergeVisibility(ctx, principal, &out); err != nil {
			return domainsuggest.ProfileAccessScope{}, err
		}
		return out, nil
	}

	scope, ok, err := p.tryPlatformProfileScope(ctx, sub)
	if err != nil {
		return domainsuggest.ProfileAccessScope{}, err
	}
	if ok {
		return scope, nil
	}

	tenantDom := principal.TenantDomain
	if tenantDom == "" {
		tenantDom = tenant.DefaultID
	}

	mobileOK, err := p.mobileSearchAllowed(ctx, sub, tenantDom)
	if err != nil {
		return domainsuggest.ProfileAccessScope{}, err
	}

	out := p.scopeOperatorAndOrg(principal, mobileOK)
	if err := p.mergeVisibility(ctx, principal, &out); err != nil {
		return domainsuggest.ProfileAccessScope{}, err
	}
	return out, nil
}

func (p *OperatingProfileAccessScopeProvider) tryPlatformProfileScope(
	ctx context.Context,
	sub string,
) (domainsuggest.ProfileAccessScope, bool, error) {
	allowed, err := p.routeAuth.AuthorizeRoute(
		ctx, sub, tenant.PlatformID, appsuggest.ResourceIAMProfileCollection, appsuggest.ActionList,
	)
	if err != nil {
		return domainsuggest.ProfileAccessScope{}, false, err
	}
	if !allowed {
		return domainsuggest.ProfileAccessScope{}, false, nil
	}
	mobileAllowed, err := p.routeAuth.AuthorizeRoute(
		ctx, sub, tenant.PlatformID, appsuggest.ResourceIAMProfileCollection, appsuggest.ActionSearchByMobile,
	)
	if err != nil {
		return domainsuggest.ProfileAccessScope{}, false, err
	}
	return domainsuggest.ProfileAccessScope{
		AllProfile: true, AllowMobileSearch: mobileAllowed,
	}, true, nil
}

func (p *OperatingProfileAccessScopeProvider) mobileSearchAllowed(ctx context.Context, sub, tenantDom string) (bool, error) {
	allowed, err := p.routeAuth.AuthorizeRoute(ctx, sub, tenantDom, appsuggest.ResourceIAMProfileCollection, appsuggest.ActionSearchByMobile)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

func (p *OperatingProfileAccessScopeProvider) scopeOperatorAndOrg(principal domainsuggest.OperatingPrincipal, mobileOK bool) domainsuggest.ProfileAccessScope {
	return domainsuggest.ProfileAccessScope{
		OperatorID:        principal.OperatorID,
		OrgIDs:            principalOrgIDs(principal),
		AllowMobileSearch: mobileOK,
	}
}

func (p *OperatingProfileAccessScopeProvider) mergeVisibility(ctx context.Context, principal domainsuggest.OperatingPrincipal, out *domainsuggest.ProfileAccessScope) error {
	if p.visibility == nil || out.AllProfile {
		return nil
	}
	ids, err := p.visibility.VisibleProfileIDs(ctx, principal)
	if err != nil {
		return err
	}
	out.ProfileIDs = mergeUniqueInt64(out.ProfileIDs, ids)
	return nil
}

func principalOrgIDs(principal domainsuggest.OperatingPrincipal) []int64 {
	if len(principal.OrgIDs) > 0 {
		return append([]int64(nil), principal.OrgIDs...)
	}
	if principal.OrgID > 0 {
		return []int64{principal.OrgID}
	}
	return nil
}

func mergeUniqueInt64(a, b []int64) []int64 {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(a)+len(b))
	for _, id := range a {
		if id <= 0 {
			continue
		}
		seen[id] = struct{}{}
	}
	for _, id := range b {
		if id <= 0 {
			continue
		}
		seen[id] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]int64, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}
