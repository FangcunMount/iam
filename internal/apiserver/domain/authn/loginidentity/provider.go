package loginidentity

// Provider 提供者，标识用户登录的身份来源，例如：用户名、手机号、微信小程序、企业微信等。
type Provider string

const (
	ProviderUsername    Provider = "username"     // 用户名提供者
	ProviderPhone       Provider = "phone"        // 手机提供者
	ProviderWechatMinip Provider = "wechat_minip" // 微信小程序提供者
	ProviderWechatOpen  Provider = "wechat_open"  // 微信开放平台提供者（微信扫码登录）
	ProviderWecom       Provider = "wecom"        // 企业微信提供者（企业微信扫码登录）
)

const (
	RealmDefault = "default" // 默认域
	RealmGlobal  = "global"  // 全局域
)

const (
	// MetaLegacyIdentifierSemantics marks identities whose identifier came from
	// the legacy OAuth credential column. That column stored either an OpenID or
	// a UnionID, so runtime compatibility must treat it as an explicitly marked
	// fallback instead of changing lookup semantics for every identity.
	MetaLegacyIdentifierSemantics = "legacy_identifier_semantics"
	LegacyIdentifierOpenOrUnion   = "openid_or_unionid"
)

// String 提供者名称
func (p Provider) String() string { return string(p) }

// Validate 验证提供者是否有效
func (p Provider) Validate() bool {
	switch p {
	case ProviderUsername, ProviderPhone, ProviderWechatMinip, ProviderWechatOpen, ProviderWecom:
		return true
	default:
		return false
	}
}
