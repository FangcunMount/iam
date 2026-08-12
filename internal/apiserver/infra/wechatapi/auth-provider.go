package wechatapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/silenceper/wechat/v2"
	"github.com/silenceper/wechat/v2/cache"
	miniConfig "github.com/silenceper/wechat/v2/miniprogram/config"
	offiaConfig "github.com/silenceper/wechat/v2/officialaccount/config"

	wechatapp "github.com/FangcunMount/iam/v3/internal/apiserver/infra/wechatapi/port"
)

// AuthProvider 微信认证服务（使用 silenceper SDK）
type AuthProvider struct {
	cache cache.Cache // SDK 使用的缓存
}

// NewAuthProvider 创建微信认证服务实例
func NewAuthProvider(cache cache.Cache) *AuthProvider {
	return &AuthProvider{
		cache: cache,
	}
}

// Code2Session 使用登录码进行登录
func (p *AuthProvider) Code2Session(ctx context.Context, appID, appSecret, jsCode string) (wechatapp.Code2SessionResult, error) {
	session := wechatapp.Code2SessionResult{}
	if appID == "" || appSecret == "" {
		return session, errors.New("appID and appSecret cannot be empty")
	}
	if jsCode == "" {
		return session, errors.New("jsCode cannot be empty")
	}

	// 调用 code2Session
	cfg := &miniConfig.Config{
		AppID:     appID,
		AppSecret: appSecret,
		Cache:     p.cache,
	}
	result, err := wechat.NewWechat().GetMiniProgram(cfg).GetAuth().Code2Session(jsCode)
	if err != nil {
		return session, fmt.Errorf("failed to call code2session: %w", err)
	}

	// 检查返回值
	if result.OpenID == "" || result.SessionKey == "" {
		return session, errors.New("empty openid or session_key returned")
	}

	// 填充结果
	session.OpenID = result.OpenID
	session.UnionID = result.UnionID
	session.SessionKey = result.SessionKey

	return session, nil
}

// DecryptPhone 解密用户手机号
func (p *AuthProvider) DecryptPhone(ctx context.Context, appID, appSecret, sessionKey, encryptedData, iv string) (wechatapp.DecryptPhoneResult, error) {
	result := wechatapp.DecryptPhoneResult{}
	if appID == "" || appSecret == "" {
		return result, errors.New("appID and appSecret cannot be empty")
	}
	if sessionKey == "" || encryptedData == "" || iv == "" {
		return result, errors.New("sessionKey, encryptedData and iv cannot be empty")
	}

	// 创建小程序实例
	wc := wechat.NewWechat()
	cfg := &miniConfig.Config{
		AppID:     appID,
		AppSecret: appSecret,
		Cache:     p.cache,
	}

	miniProgram := wc.GetMiniProgram(cfg)
	encryptor := miniProgram.GetEncryptor()

	// 解密数据
	plainData, err := encryptor.Decrypt(sessionKey, encryptedData, iv)
	if err != nil {
		return result, fmt.Errorf("failed to decrypt data: %w", err)
	}

	// 解析手机号
	type PhoneInfo struct {
		PhoneNumber     string `json:"phoneNumber"`
		PurePhoneNumber string `json:"purePhoneNumber"`
		CountryCode     string `json:"countryCode"`
	}

	var phoneInfo PhoneInfo
	if err := json.Unmarshal([]byte(plainData.PhoneNumber), &phoneInfo); err != nil {
		// 如果解析失败，尝试直接使用 PhoneNumber 字段
		result.PhoneNumber = plainData.PhoneNumber
		return result, nil
	}

	result.PhoneNumber = phoneInfo.PhoneNumber
	result.PurePhoneNumber = phoneInfo.PurePhoneNumber
	result.CountryCode = phoneInfo.CountryCode

	return result, nil
}

// ExchangeOAuthCode 用授权 code 换取 openid（网站应用微信登录 / 公众号网页授权共用 sns 接口）。
// 文档: https://developers.weixin.qq.com/doc/oplatform/Website_App/WeChat_Login/Wechat_Login.html
func (p *AuthProvider) ExchangeOAuthCode(ctx context.Context, appID, appSecret, code string) (wechatapp.OpenOAuthResult, error) {
	result := wechatapp.OpenOAuthResult{}
	if appID == "" || appSecret == "" {
		return result, errors.New("appID and appSecret cannot be empty")
	}
	if code == "" {
		return result, errors.New("code cannot be empty")
	}

	// 通过code获取access_token
	cfg := &offiaConfig.Config{
		AppID:     appID,
		AppSecret: appSecret,
		Cache:     p.cache,
	}
	accessToken, err := wechat.NewWechat().GetOfficialAccount(cfg).GetOauth().GetUserAccessTokenContext(ctx, code)
	if err != nil {
		return result, fmt.Errorf("failed to get access token: %w", err)
	}

	// 填充结果
	result.AccessToken = accessToken.AccessToken
	result.ExpiresIn = accessToken.ExpiresIn
	result.RefreshToken = accessToken.RefreshToken
	result.OpenID = accessToken.OpenID
	result.Scope = accessToken.Scope
	result.UnionID = accessToken.UnionID

	return result, nil
}
