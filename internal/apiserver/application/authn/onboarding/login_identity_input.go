package onboarding

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// OnboardingLoginIdentityInput 登录身份输入。
//
// 调用点根据开通入口选择具体输入类型；onboarding 固定流程只依赖该接口产出的
// ProviderKey 和创建标记，不再二次解析账号类型。
type OnboardingLoginIdentityInput interface {
	prepareOnboardingLoginIdentity(context.Context, loginIdentityPrepareDeps, OnboardingUserInput) (preparedLoginIdentity, error)
}

// loginIdentityPrepareDeps 登录身份准备依赖。
type loginIdentityPrepareDeps struct {
	wechatIdentityResolver *wechatIdentityResolver
}

// preparedLoginIdentity 准备好的登录身份。
type preparedLoginIdentity struct {
	ProviderKey            loginidentity.ProviderKey
	Profile                map[string]string
	Meta                   map[string]string
	NeedPasswordCredential bool
	AllowUserRepair        bool
}

// prepareOnboardingLoginIdentity 准备登录身份。
func (i UsernameLoginIdentityInput) prepareOnboardingLoginIdentity(_ context.Context, _ loginIdentityPrepareDeps, user OnboardingUserInput) (preparedLoginIdentity, error) {
	identifier := usernameIdentifier(user, i.Username)
	return preparedFromProviderKey(loginidentity.UsernameProviderKey(i.RealmTenantID, identifier), i.Profile, i.Meta, true, false)
}

// prepareOnboardingLoginIdentity 准备登录身份。
func (i MockConsumerUsernameLoginIdentityInput) prepareOnboardingLoginIdentity(_ context.Context, _ loginIdentityPrepareDeps, user OnboardingUserInput) (preparedLoginIdentity, error) {
	identifier := usernameIdentifier(user, i.Username)
	return preparedFromProviderKey(loginidentity.MockConsumerProviderKey(identifier), i.Profile, i.Meta, true, false)
}

// prepareOnboardingLoginIdentity 准备登录身份。
func (i WechatMiniLoginIdentityInput) prepareOnboardingLoginIdentity(ctx context.Context, deps loginIdentityPrepareDeps, _ OnboardingUserInput) (preparedLoginIdentity, error) {
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

// preparedFromProviderKey 从提供者键准备登录身份。
func preparedFromProviderKey(
	key loginidentity.ProviderKey,
	profile map[string]string,
	metaData map[string]string,
	needPasswordCredential bool,
	allowUserRepair bool,
) (preparedLoginIdentity, error) {
	if !key.IsValid() {
		return preparedLoginIdentity{}, perrors.WithCode(code.ErrInvalidArgument, "login identity provider key is incomplete")
	}
	return preparedLoginIdentity{
		ProviderKey:            key,
		Profile:                cloneStringMap(profile),
		Meta:                   cloneStringMap(metaData),
		NeedPasswordCredential: needPasswordCredential,
		AllowUserRepair:        allowUserRepair,
	}, nil
}

// usernameIdentifier 用户名标识符。
func usernameIdentifier(user OnboardingUserInput, explicitUsername string) string {
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

// trimmed 修剪微信小程序登录身份输入。
func (i WechatMiniLoginIdentityInput) trimmed() WechatMiniLoginIdentityInput {
	i.AppID = trimStringPtr(i.AppID)
	i.JsCode = trimStringPtr(i.JsCode)
	i.OpenID = trimStringPtr(i.OpenID)
	i.UnionID = trimStringPtr(i.UnionID)
	i.Profile = cloneStringMap(i.Profile)
	i.Meta = cloneStringMap(i.Meta)
	return i
}

// withResolvedIdentity 使用解析好的身份填充微信小程序登录身份输入。
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
