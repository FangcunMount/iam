package authentication

// AMR（认证方法引用），用于审计与 Step-Up
type AMR string

const (
	AMRPassword AMR = "pwd"
	AMROTP      AMR = "otp"
	AMRWx       AMR = "wechat"
	AMRWecom    AMR = "wecom"
)

// CredentialKind 认证凭据类型
type CredentialKind string

const (
	CredentialKindPassword    CredentialKind = "password"
	CredentialKindPhoneOTP    CredentialKind = "phone_otp"
	CredentialKindWechatMinip CredentialKind = "oauth_wx_minip"
	CredentialKindWecom       CredentialKind = "oauth_wecom"
)
