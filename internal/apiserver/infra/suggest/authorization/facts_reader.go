package authorization

import (
	"context"
	"strconv"

	authorizationapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	appsuggest "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
	"github.com/FangcunMount/iam/v3/pkg/tenant"
)

// FactsReader 从 AuthZ 查询 Suggest 授权事实。
type FactsReader struct {
	permissions authorizationapp.RoutePermissionChecker
}

// NewFactsReader 创建 reader。
func NewFactsReader(permissions authorizationapp.RoutePermissionChecker) *FactsReader {
	return &FactsReader{permissions: permissions}
}

// ReadAuthorizationFacts 实现 queryprofile.AuthorizationFactsReader。
func (r *FactsReader) ReadAuthorizationFacts(
	ctx context.Context,
	principal visibility.Principal,
) (visibility.AuthorizationFacts, error) {
	if r == nil || r.permissions == nil {
		return visibility.AuthorizationFacts{}, nil
	}
	sub := "user:" + strconv.FormatInt(principal.OperatorID, 10)

	listAllowed, err := r.permissions.CheckRoutePermission(
		ctx, sub, tenant.PlatformID, appsuggest.ResourceIAMProfileCollection, appsuggest.ActionList,
	)
	if err != nil {
		return visibility.AuthorizationFacts{}, err
	}
	if listAllowed {
		mobileAllowed, err := r.permissions.CheckRoutePermission(
			ctx, sub, tenant.PlatformID, appsuggest.ResourceIAMProfileCollection, appsuggest.ActionSearchByMobile,
		)
		if err != nil {
			return visibility.AuthorizationFacts{}, err
		}
		return visibility.AuthorizationFacts{
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
		return visibility.AuthorizationFacts{}, err
	}
	return visibility.AuthorizationFacts{TenantMobileSearchAllowed: mobileOK}, nil
}

var _ appquery.AuthorizationFactsReader = (*FactsReader)(nil)
