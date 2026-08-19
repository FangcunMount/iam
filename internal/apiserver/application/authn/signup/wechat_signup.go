package signup

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authnexternal "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/externalidentity"
	idpresolver "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
	loginidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	idpidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

type externalIdentityResolver interface {
	Resolve(context.Context, idpresolver.ResolveRequest) (idpidentity.ExternalIdentity, error)
}

// prepareSignupLoginIdentity prepares a mini-program login identity outside the local transaction.
func (i WechatMiniLoginIdentityInput) prepareSignupLoginIdentity(ctx context.Context, deps loginIdentityPrepareDeps, _ SignupUserInput) (preparedLoginIdentity, error) {
	input := i.trimmed()
	if strings.TrimSpace(valueOfStringPtr(input.OpenID)) != "" {
		prepared, err := preparedFromProviderKey(
			loginidentity.WechatMinipProviderKey(
				valueOfStringPtr(input.AppID),
				valueOfStringPtr(input.OpenID),
				valueOfStringPtr(input.UnionID),
			),
			input.Profile,
			input.Meta,
			false,
			true,
		)
		if err != nil {
			return preparedLoginIdentity{}, err
		}
		prepared.Source = loginIdentitySourceTrustedLegacyInput
		return prepared, nil
	}

	if strings.TrimSpace(valueOfStringPtr(input.AppID)) == "" || strings.TrimSpace(valueOfStringPtr(input.JsCode)) == "" {
		return preparedLoginIdentity{}, perrors.WithCode(code.ErrInvalidArgument, "appid and jscode are required for wechat mini program")
	}
	if deps.externalIdentityResolver == nil {
		return preparedLoginIdentity{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app configuration service not available")
	}

	identity, err := deps.externalIdentityResolver.Resolve(ctx, idpresolver.ResolveRequest{
		Provider: idpidentity.ProviderWechatMinip,
		Realm:    valueOfStringPtr(input.AppID),
		Code:     valueOfStringPtr(input.JsCode),
	})
	if err != nil {
		return preparedLoginIdentity{}, authnexternal.MapSignupError(err)
	}
	key, err := authnexternal.ProviderKey(identity)
	if err != nil {
		return preparedLoginIdentity{}, perrors.WithCode(code.ErrInvalidCredential, "failed to map wechat external identity: %v", err)
	}
	prepared, err := preparedFromProviderKey(key, input.Profile, input.Meta, false, true)
	if err != nil {
		return preparedLoginIdentity{}, err
	}
	prepared.Source = loginIdentitySourceProviderVerified
	return prepared, nil
}

func (i WechatMiniLoginIdentityInput) trimmed() WechatMiniLoginIdentityInput {
	i.AppID = trimStringPtr(i.AppID)
	i.JsCode = trimStringPtr(i.JsCode)
	i.OpenID = trimStringPtr(i.OpenID)
	i.UnionID = trimStringPtr(i.UnionID)
	i.Profile = cloneStringMap(i.Profile)
	i.Meta = cloneStringMap(i.Meta)
	return i
}
