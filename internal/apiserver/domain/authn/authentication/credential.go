package authentication

// CredentialKind is the authentication proof routing key.
//
// It is intentionally separate from persistent Credential record types: phone OTP,
// WeChat, and WeCom are login proofs, not persistent Credential records.
type CredentialKind string

const (
	CredentialKindPassword    CredentialKind = "password"
	CredentialKindPhoneOTP    CredentialKind = "phone_otp"
	CredentialKindWechatMinip CredentialKind = "oauth_wx_minip"
	CredentialKindWecom       CredentialKind = "oauth_wecom"
)

type AuthCredential interface {
	CredentialKind() CredentialKind
}
