package authentication

import "time"

// Method 表示 IAM 实际执行的认证策略。
type Method string

const (
	MethodPassword    Method = "password"
	MethodPhoneOTP    Method = "phone_otp"
	MethodWechatMinip Method = "wechat_minip"
	MethodWechatOpen  Method = "oauth_wx_open"
	MethodWecom       Method = "wecom"
)

// AMR（认证方法引用），用于审计与对外认证手段表达。
type AMR string

const (
	AMRPassword AMR = "pwd"         // 密码认证
	AMROTP      AMR = "otp"         // 短信验证码认证
	AMRWx       AMR = "wechat"      // 微信认证（微信小程序认证）
	AMRWxOpen   AMR = "wechat_open" // 微信开放平台认证（微信扫码登录）
	AMRWecom    AMR = "wecom"       // 企业微信认证（企业微信扫码登录）
)

// AuthenticationContext 是一次认证成功后的领域上下文集中表达。
// Method 表示 IAM 实际策略；Realm 表示 provider 身份命名空间；
// AMR 是可对外表达的认证手段；AuthenticatedAt 是原始认证时间。
type AuthenticationContext struct {
	Method          Method
	Realm           string
	AMR             []AMR
	AuthenticatedAt time.Time
}

// Clone 返回防御性副本。
func (c AuthenticationContext) Clone() AuthenticationContext {
	out := c
	if len(c.AMR) > 0 {
		out.AMR = append([]AMR(nil), c.AMR...)
	}
	return out
}

// AMRStrings 返回 AMR 的字符串切片副本。
func (c AuthenticationContext) AMRStrings() []string {
	if len(c.AMR) == 0 {
		return nil
	}
	out := make([]string, len(c.AMR))
	for i, item := range c.AMR {
		out[i] = string(item)
	}
	return out
}

// NewAuthenticationContext 构造一次刚刚成功的认证上下文。
// 新认证若未显式传入时间，则使用当前时间；恢复历史状态应使用
// RestoreAuthenticationContext，避免把未知的历史认证时间误写为现在。
func NewAuthenticationContext(method Method, realm string, amr []AMR, authenticatedAt time.Time) AuthenticationContext {
	if authenticatedAt.IsZero() {
		authenticatedAt = time.Now().UTC()
	} else {
		authenticatedAt = authenticatedAt.UTC()
	}
	return AuthenticationContext{
		Method:          method,
		Realm:           realm,
		AMR:             append([]AMR(nil), amr...),
		AuthenticatedAt: authenticatedAt,
	}
}

// RestoreAuthenticationContext 从持久化事实恢复认证上下文。
// authenticatedAt 为零表示历史认证时间未知，不能被提升为当前时间。
func RestoreAuthenticationContext(method Method, realm string, amr []AMR, authenticatedAt time.Time) AuthenticationContext {
	if !authenticatedAt.IsZero() {
		authenticatedAt = authenticatedAt.UTC()
	}
	return AuthenticationContext{
		Method:          method,
		Realm:           realm,
		AMR:             append([]AMR(nil), amr...),
		AuthenticatedAt: authenticatedAt,
	}
}

// CredentialKind 认证凭据类型
type CredentialKind string

const (
	CredentialKindPassword    CredentialKind = "password"       // 密码认证
	CredentialKindPhoneOTP    CredentialKind = "phone_otp"      // 短信验证码认证
	CredentialKindWechatMinip CredentialKind = "oauth_wx_minip" // 微信小程序认证
	CredentialKindWechatOpen  CredentialKind = "oauth_wx_open"  // 微信开放平台认证（微信扫码登录）
	CredentialKindWecom       CredentialKind = "oauth_wecom"    // 企业微信认证（企业微信扫码登录）
)
