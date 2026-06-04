package authentication

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
)

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
	return repo.FindLoginIdentityByGlobalIdentifier(ctx, provider, unionID)
}
