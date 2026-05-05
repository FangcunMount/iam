package login

// PasswordWirePayload is the v2 password method_payload shape.
type PasswordWirePayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TenantID uint64 `json:"tenant_id,omitempty"`
}

// PhoneOTPWirePayload is the v2 phone OTP method_payload shape.
type PhoneOTPWirePayload struct {
	Phone   string `json:"phone"`
	OTPCode string `json:"otp_code"`
}

// WechatWirePayload is the v2 WeChat mini-program method_payload shape.
type WechatWirePayload struct {
	AppID string `json:"app_id"`
	Code  string `json:"code"`
}

// WecomWirePayload is the v2 WeCom method_payload shape.
type WecomWirePayload struct {
	CorpID   string `json:"corp_id"`
	AuthCode string `json:"auth_code"`
}
