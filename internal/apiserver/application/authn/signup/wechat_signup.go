package signup

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	loginidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	idpPort "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// prepareSignupLoginIdentity 准备登录身份。
// 参数：
//   - ctx: 上下文
//   - deps: 准备依赖
//   - _: 用户输入
//
// 返回：
//   - preparedLoginIdentity: 准备后的登录身份
//   - error: 错误
func (i WechatMiniLoginIdentityInput) prepareSignupLoginIdentity(ctx context.Context, deps loginIdentityPrepareDeps, _ SignupUserInput) (preparedLoginIdentity, error) {
	input := i.trimmed()
	if strings.TrimSpace(valueOfStringPtr(input.OpenID)) == "" {
		if deps.wechatIdentityResolver == nil {
			return preparedLoginIdentity{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app configuration service not available")
		}
		identity, err := deps.wechatIdentityResolver.ResolveMiniProgram(ctx, input)
		if err != nil {
			return preparedLoginIdentity{}, err
		}
		input = input.withResolvedIdentity(identity)
	}

	return preparedFromProviderKey(
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
}

// trimmed 修剪登录身份输入。
func (i WechatMiniLoginIdentityInput) trimmed() WechatMiniLoginIdentityInput {
	i.AppID = trimStringPtr(i.AppID)
	i.JsCode = trimStringPtr(i.JsCode)
	i.OpenID = trimStringPtr(i.OpenID)
	i.UnionID = trimStringPtr(i.UnionID)
	i.Profile = cloneStringMap(i.Profile)
	i.Meta = cloneStringMap(i.Meta)
	return i
}

// withResolvedIdentity 使用解析后的身份信息更新登录身份输入。
func (i WechatMiniLoginIdentityInput) withResolvedIdentity(identity wechatIdentity) WechatMiniLoginIdentityInput {
	if identity.OpenID == "" {
		return i
	}
	if i.OpenID == nil {
		i.OpenID = stringPtr(identity.OpenID)
	}
	if identity.UnionID != "" && i.UnionID == nil {
		i.UnionID = stringPtr(identity.UnionID)
	}
	i.JsCode = nil
	return i
}

// wechatIdentityResolver 微信身份解析器。
type wechatIdentityResolver struct {
	idp              authentication.IdentityProvider
	wechatAppQuerier idpPort.Repository
	secretVault      idpPort.SecretVault
}

// wechatIdentity 微信身份。
type wechatIdentity struct {
	OpenID  string
	UnionID string
}

// newWechatIdentityResolver 创建微信身份解析器。
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

// ResolveMiniProgram 解析微信小程序身份。
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

// checkMiniProgramIdentityInfo 检查微信小程序身份信息。
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

// resolveAppSecret 解析微信小程序应用密钥。
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

// checkMiniProgramAppInfo 检查微信小程序应用信息。
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

// stringPtr 转换字符串为指针。
func stringPtr(v string) *string {
	return &v
}
