package wechat

import (
	"context"
	"fmt"

	"github.com/silenceper/wechat/v2"
	"github.com/silenceper/wechat/v2/cache"
	workConfig "github.com/silenceper/wechat/v2/work/config"

	externalidentity "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
	wechatAuthPort "github.com/FangcunMount/iam/v3/internal/apiserver/infra/wechatapi/port"
)

// IdentityProviderImpl 微信身份提供商的实现
// - 微信小程序登录：委托 IDP 模块提供的 AuthProvider 调用微信接口
// - 企业微信登录：暂时保留 silenceper SDK 实现
type IdentityProviderImpl struct {
	auth  wechatAuthPort.AuthProvider
	cache cache.Cache
}

// 确保实现了接口
var _ externalidentity.ProviderExchanger = (*IdentityProviderImpl)(nil)

// NewIdentityProvider 创建微信身份提供商。
// auth 同时覆盖小程序 code2Session 与开放平台 OAuth code 换取（同一 wechatapi.AuthProvider 实现）。
func NewIdentityProvider(auth wechatAuthPort.AuthProvider, cache cache.Cache) *IdentityProviderImpl {
	return &IdentityProviderImpl{
		auth:  auth,
		cache: cache,
	}
}

// ExchangeWxMinipCode 微信小程序 jsCode 换取 session
// 文档: https://developers.weixin.qq.com/miniprogram/dev/OpenApiDoc/user-login/code2Session.html
func (p *IdentityProviderImpl) ExchangeWxMinipCode(ctx context.Context, appID, appSecret, jsCode string) (openID, unionID string, err error) {
	if p.auth == nil {
		return "", "", fmt.Errorf("wechat auth provider is not configured")
	}

	result, err := p.auth.Code2Session(ctx, appID, appSecret, jsCode)
	if err != nil {
		return "", "", fmt.Errorf("failed to call code2session: %w", err)
	}
	if result.OpenID == "" {
		return "", "", fmt.Errorf("openid is empty in code2session result")
	}
	return result.OpenID, result.UnionID, nil
}

// ExchangeWecomCode 企业微信 code 换取用户信息
// 文档: https://developer.work.weixin.qq.com/document/path/91023
func (p *IdentityProviderImpl) ExchangeWecomCode(ctx context.Context, corpID, agentID, corpSecret, code string) (openUserID, userID string, err error) {
	// 创建企业微信实例（依赖 silenceper SDK）
	cfg := &workConfig.Config{
		CorpID:     corpID,
		CorpSecret: corpSecret,
		AgentID:    agentID,
		Cache:      p.cache,
	}
	workApp := wechat.NewWechat().GetWork(cfg)

	// 获取用户信息
	userInfo, err := workApp.GetOauth().GetUserInfo(code)
	if err != nil {
		return "", "", fmt.Errorf("failed to get wecom user info: %w", err)
	}

	return userInfo.OpenID, userInfo.UserID, nil
}

// ExchangeWxOpenCode 微信开放平台/网站应用扫码登录：code 换 openid/unionid。
// 文档: https://developers.weixin.qq.com/doc/oplatform/Website_App/WeChat_Login/Wechat_Login.html
func (p *IdentityProviderImpl) ExchangeWxOpenCode(ctx context.Context, appID, appSecret, code string) (openID, unionID string, err error) {
	if p.auth == nil {
		return "", "", fmt.Errorf("wechat open auth provider is not configured")
	}

	result, err := p.auth.ExchangeOAuthCode(ctx, appID, appSecret, code)
	if err != nil {
		return "", "", fmt.Errorf("failed to exchange oauth code: %w", err)
	}
	if result.OpenID == "" {
		return "", "", fmt.Errorf("openid is empty in oauth code exchange result")
	}
	return result.OpenID, result.UnionID, nil
}
