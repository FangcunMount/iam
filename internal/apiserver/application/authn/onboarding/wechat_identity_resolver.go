package onboarding

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	idpPort "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

type wechatIdentityResolver struct {
	idp              authentication.IdentityProvider
	wechatAppQuerier idpPort.Repository
	secretVault      idpPort.SecretVault
}

type wechatIdentity struct {
	OpenID  string
	UnionID string
}

// newWechatIdentityResolver 创建微信小程序身份解析器
func newWechatIdentityResolver(
	idp authentication.IdentityProvider,
	wechatAppQuerier idpPort.Repository,
	secretVault idpPort.SecretVault,
) *wechatIdentityResolver {
	return &wechatIdentityResolver{
		idp:              idp,
		wechatAppQuerier: wechatAppQuerier,
		secretVault:      secretVault,
	}
}

// ResolveMiniProgram 在事务外把微信小程序输入解析为稳定的 openid/unionid。
func (r *wechatIdentityResolver) ResolveMiniProgram(ctx context.Context, input WechatMiniLoginIdentityInput) (wechatIdentity, error) {
	input = input.trimmed()
	identity, err := r.checkMiniProgramIdentityInfo(input)
	if err != nil {
		return wechatIdentity{}, err
	}
	if identity.OpenID != "" {
		return identity, nil
	}
	if r == nil || r.idp == nil || r.wechatAppQuerier == nil || r.secretVault == nil {
		return wechatIdentity{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app configuration service not available")
	}

	appID := valueOfStringPtr(input.AppID)
	jsCode := valueOfStringPtr(input.JsCode)
	appSecret, err := r.resolveAppSecret(ctx, appID)
	if err != nil {
		return wechatIdentity{}, err
	}

	openID, unionID, err := r.idp.ExchangeWxMinipCode(ctx, appID, appSecret, jsCode)
	if err != nil {
		return wechatIdentity{}, perrors.WithCode(code.ErrInvalidCredential, "failed to call wechat code2session: %v", err)
	}

	return wechatIdentity{OpenID: openID, UnionID: unionID}, nil
}

func (r *wechatIdentityResolver) checkMiniProgramIdentityInfo(input WechatMiniLoginIdentityInput) (wechatIdentity, error) {
	openID := strings.TrimSpace(valueOfStringPtr(input.OpenID))
	if openID != "" {
		return wechatIdentity{
			OpenID:  openID,
			UnionID: strings.TrimSpace(valueOfStringPtr(input.UnionID)),
		}, nil
	}
	if strings.TrimSpace(valueOfStringPtr(input.AppID)) == "" || strings.TrimSpace(valueOfStringPtr(input.JsCode)) == "" {
		return wechatIdentity{}, perrors.WithCode(code.ErrInvalidArgument, "appid and jscode are required for wechat mini program")
	}
	return wechatIdentity{}, nil
}

func (r *wechatIdentityResolver) resolveAppSecret(ctx context.Context, appID string) (string, error) {
	appSecretCipher, err := r.checkMiniProgramAppInfo(ctx, appID)
	if err != nil {
		return "", err
	}
	plaintext, err := r.secretVault.Decrypt(ctx, appSecretCipher)
	if err != nil {
		return "", perrors.WithCode(code.ErrInvalidArgument, "failed to decrypt wechat app secret: %v", err)
	}
	return string(plaintext), nil
}

func (r *wechatIdentityResolver) checkMiniProgramAppInfo(ctx context.Context, appID string) ([]byte, error) {
	wechatApp, err := r.wechatAppQuerier.GetByAppID(ctx, appID)
	if err != nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "failed to query wechat app: %v", err)
	}
	if wechatApp == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat app not found: %s", appID)
	}
	if !wechatApp.IsEnabled() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat app is disabled: %s", appID)
	}
	if wechatApp.Cred == nil || wechatApp.Cred.Auth == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat app credentials not found")
	}
	return wechatApp.Cred.Auth.AppSecretCipher, nil
}

func stringPtr(v string) *string {
	return &v
}
