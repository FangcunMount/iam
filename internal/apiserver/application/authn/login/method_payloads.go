package login

import "github.com/FangcunMount/iam/internal/pkg/meta"

// MethodPayload 是按认证方式拆分后的应用层输入。
type MethodPayload interface {
	methodPayload()
	commonPayload() methodPayloadCommon
}

type methodPayloadCommon struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string
}

func (p methodPayloadCommon) commonPayload() methodPayloadCommon {
	return p
}

func commonPayloadFromRequest(req LoginRequest) methodPayloadCommon {
	return methodPayloadCommon{TenantID: req.TenantID}
}

type PasswordPayload struct {
	methodPayloadCommon
	Username string
	Password string
}

func (PasswordPayload) methodPayload() {}

type PhoneOTPPayload struct {
	methodPayloadCommon
	PhoneE164 string
	OTP       string
}

func (PhoneOTPPayload) methodPayload() {}

type WechatMiniPayload struct {
	methodPayloadCommon
	AppID  string
	JSCode string
}

func (WechatMiniPayload) methodPayload() {}

type WecomPayload struct {
	methodPayloadCommon
	CorpID string
	Code   string
}

func (WecomPayload) methodPayload() {}

type BearerPayload struct {
	methodPayloadCommon
	Token string
}

func (BearerPayload) methodPayload() {}
