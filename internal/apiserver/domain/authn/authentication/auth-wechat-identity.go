package authentication

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
)

type legacyWechatIdentityRepository interface {
	FindLegacyWechatIdentityByProviderKey(
		ctx context.Context,
		provider loginidentity.Provider,
		realm string,
		identifier string,
	) (*LoginIdentityLookup, error)
}

func findWechatIdentityByOpenIDThenUnionID(
	ctx context.Context,
	repo LoginIdentityRepository,
	provider loginidentity.Provider,
	appID string,
	openID string,
	unionID string,
) (*LoginIdentityLookup, error) {
	lookup, err := repo.FindLoginIdentityByProviderKey(ctx, provider, appID, openID)
	if err != nil || lookup != nil {
		return lookup, err
	}
	if unionID == "" {
		return nil, nil
	}
	lookup, err = repo.FindLoginIdentityByGlobalIdentifier(ctx, provider, unionID)
	if err != nil || lookup != nil {
		return lookup, err
	}
	legacyRepo, ok := repo.(legacyWechatIdentityRepository)
	if !ok {
		return nil, nil
	}
	return legacyRepo.FindLegacyWechatIdentityByProviderKey(ctx, provider, appID, unionID)
}
