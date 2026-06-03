package authentication

// AMR（认证方法引用），用于审计与 Step-Up
type AMR string

const (
	AMRPassword AMR = "pwd"         // 密码认证
	AMROTP      AMR = "otp"         // 短信验证码认证
	AMRWx       AMR = "wechat"      // 微信认证（微信小程序认证）
	AMRWxOpen   AMR = "wechat_open" // 微信开放平台认证（微信扫码登录）
	AMRWecom    AMR = "wecom"       // 企业微信认证（企业微信扫码登录）
)

// CredentialKind 认证凭据类型
type CredentialKind string

const (
	CredentialKindPassword    CredentialKind = "password"       // 密码认证
	CredentialKindPhoneOTP    CredentialKind = "phone_otp"      // 短信验证码认证
	CredentialKindWechatMinip CredentialKind = "oauth_wx_minip" // 微信小程序认证
	CredentialKindWechatOpen  CredentialKind = "oauth_wx_open"  // 微信开放平台认证（微信扫码登录）
	CredentialKindWecom       CredentialKind = "oauth_wecom"    // 企业微信认证（企业微信扫码登录）
)
