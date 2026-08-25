package signup

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// loginIdentityPrepareDeps 登录身份准备依赖。
type loginIdentityPrepareDeps struct {
	externalIdentityResolver externalIdentityResolver
}

type loginIdentitySource uint8

const (
	loginIdentitySourceLocal loginIdentitySource = iota + 1
	loginIdentitySourceProviderVerified
	loginIdentitySourceTrustedLegacyInput
)

// preparedLoginIdentity 准备后的登录身份。
type preparedLoginIdentity struct {
	ProviderKey            loginidentity.ProviderKey
	Profile                map[string]string
	Meta                   map[string]string
	NeedPasswordCredential bool
	AllowUserRepair        bool
	Source                 loginIdentitySource
}

// prepareSignupLoginIdentity 准备登录身份。
func (i UsernameLoginIdentityInput) prepareSignupLoginIdentity(_ context.Context, _ loginIdentityPrepareDeps, user SignupUserInput) (preparedLoginIdentity, error) {
	identifier := usernameIdentifier(user, i.Username)
	key, err := loginidentity.NewUsernameProviderKey(i.RealmTenantID, identifier)
	if err != nil {
		return preparedLoginIdentity{}, incompleteProviderKeyError()
	}
	return preparedFromProviderKey(key, i.Profile, i.Meta, true, false)
}

// preparedFromProviderKey 从提供者密钥准备登录身份。
func preparedFromProviderKey(
	key loginidentity.ProviderKey,
	profile map[string]string,
	metaData map[string]string,
	needPasswordCredential bool,
	allowUserRepair bool,
) (preparedLoginIdentity, error) {
	if !key.IsValid() {
		return preparedLoginIdentity{}, incompleteProviderKeyError()
	}
	return preparedLoginIdentity{
		ProviderKey:            key,
		Profile:                cloneStringMap(profile),
		Meta:                   cloneStringMap(metaData),
		NeedPasswordCredential: needPasswordCredential,
		AllowUserRepair:        allowUserRepair,
		Source:                 loginIdentitySourceLocal,
	}, nil
}

func incompleteProviderKeyError() error {
	return perrors.WithCode(code.ErrInvalidArgument, "login identity provider key is incomplete")
}

// usernameIdentifier 用户名标识符。
func usernameIdentifier(user SignupUserInput, explicitUsername string) string {
	if username := strings.TrimSpace(explicitUsername); username != "" {
		return username
	}
	if !user.Email.IsEmpty() {
		return strings.TrimSpace(user.Email.String())
	}
	if !user.Phone.IsEmpty() {
		return strings.TrimSpace(user.Phone.String())
	}
	return ""
}
