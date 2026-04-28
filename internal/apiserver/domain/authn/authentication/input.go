package authentication

import "github.com/FangcunMount/iam/internal/pkg/meta"

// AuthInput 统一认证输入参数（应用层 -> 领域层）
type AuthInput struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string

	// password
	Username string
	Password string

	// phone_otp
	PhoneE164 string
	OTP       string

	// wx_minip
	WxAppID     string
	WxAppSecret string
	WxJsCode    string

	// wecom
	WecomCorpID     string
	WecomAgentID    string
	WecomCorpSecret string
	WecomCode       string
	WecomState      string

	// jwt_token
	AccessToken string
}

type ErrCode string

const (
	ErrInvalidCredential  ErrCode = "invalid_credential"
	ErrOTPMissingOrExpiry ErrCode = "otp_invalid_or_expired"
	ErrStateMismatch      ErrCode = "state_mismatch"
	ErrIDPExchangeFailed  ErrCode = "idp_exchange_failed"
	ErrNoBinding          ErrCode = "no_binding"
	ErrLocked             ErrCode = "locked"
	ErrDisabled           ErrCode = "disabled"
)

// 策略的判决单（业务失败走 ErrCode，系统异常用 error）
type AuthDecision struct {
	OK           bool
	ErrCode      ErrCode
	Principal    *Principal // OK=true 时有效
	CredentialID meta.ID    // 命中的凭据ID（给应用层记成功/失败/锁定）

	// 可选：比如密码条件再哈希
	ShouldRotate bool
	NewMaterial  []byte
	NewAlgo      *string
}
