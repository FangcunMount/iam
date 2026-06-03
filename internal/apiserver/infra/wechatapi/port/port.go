package port

import "context"

// AuthProvider 微信认证服务
// 职责：提供微信认证服务，包括小程序 code2Session、手机号解密、开放平台 OAuth code 换取
type AuthProvider interface {
	Code2Session(ctx context.Context, appID, appSecret, jsCode string) (Code2SessionResult, error)
	DecryptPhone(ctx context.Context, appID, appSecret, sessionKey, encryptedData, iv string) (DecryptPhoneResult, error)
	// ExchangeOAuthCode 网站应用/开放平台扫码登录：用授权 code 换取用户 openid（及 sns access_token）。
	ExchangeOAuthCode(ctx context.Context, appID, appSecret, code string) (OpenOAuthResult, error)
}

// Code2SessionResult 小程序 code2Session 结果
type Code2SessionResult struct {
	OpenID     string
	UnionID    string
	SessionKey string
}

// DecryptPhoneResult 手机号解密结果
type DecryptPhoneResult struct {
	PhoneNumber     string
	PurePhoneNumber string
	CountryCode     string
}

// OpenOAuthResult 开放平台 OAuth code 换取结果
// 文档: https://developers.weixin.qq.com/doc/oplatform/Website_App/WeChat_Login/Wechat_Login.html
type OpenOAuthResult struct {
	AccessToken  string // 访问令牌
	ExpiresIn    int64  // 过期时间
	RefreshToken string // 刷新令牌
	OpenID       string // 用户 OpenID
	Scope        string // 权限范围
	UnionID      string // 用户 UnionID
}
