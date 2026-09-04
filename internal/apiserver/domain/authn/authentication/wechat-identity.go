package authentication

import (
	"context"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
)

// legacyWechatIdentityRepository 微信开放平台身份仓储接口
type legacyWechatIdentityRepository interface {
	// FindLegacyWechatIdentityByProviderKey 根据 provider key 查找登录身份
	FindLegacyWechatIdentityByProviderKey(
		ctx context.Context,
		provider loginidentity.Provider,
		realm string,
		identifier string,
	) (*LoginIdentityLookup, error)
}

// findWechatIdentityByOpenIDThenUnionID 根据 openID 查找登录身份，必要时用 unionID 回退。
func findWechatIdentityByOpenIDThenUnionID(
	ctx context.Context,
	repo LoginIdentityRepository,
	provider loginidentity.Provider,
	appID string,
	openID string,
	unionID string,
) (*LoginIdentityLookup, error) {
	// 根据 openID 查找登录身份
	lookup, err := repo.FindLoginIdentityByProviderKey(ctx, provider, appID, openID)
	if err != nil || lookup != nil {
		return lookup, err
	}
	// 如果 unionID 为空，则返回空
	if unionID == "" {
		return nil, nil
	}

	// 根据 unionID 查找登录身份
	lookup, err = repo.FindLoginIdentityByGlobalIdentifier(ctx, provider, unionID)
	if err != nil || lookup != nil {
		return lookup, err
	}
	// 如果 repo 是 legacyWechatIdentityRepository，则使用 legacyWechatIdentityRepository 查找登录身份
	legacyRepo, ok := repo.(legacyWechatIdentityRepository)
	if !ok {
		return nil, nil
	}

	// 使用 legacyWechatIdentityRepository 查找登录身份
	return legacyRepo.FindLegacyWechatIdentityByProviderKey(
		ctx,
		provider,
		appID,
		unionID,
	)
}
