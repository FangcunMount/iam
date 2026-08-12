package signup

import (
	"context"

	loginidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
)

func (i MockConsumerUsernameLoginIdentityInput) prepareSignupLoginIdentity(_ context.Context, _ loginIdentityPrepareDeps, user SignupUserInput) (preparedLoginIdentity, error) {
	identifier := usernameIdentifier(user, i.Username)
	return preparedFromProviderKey(loginidentity.MockConsumerProviderKey(identifier), i.Profile, i.Meta, true, false)
}
