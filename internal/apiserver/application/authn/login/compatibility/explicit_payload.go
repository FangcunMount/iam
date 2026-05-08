package compatibility

import (
	"encoding/json"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/method"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// PasswordWirePayload 密码登录负载
type PasswordWirePayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TenantID uint64 `json:"tenant_id,omitempty"`
}

// PhoneOTPWirePayload 手机号OTP登录负载
type PhoneOTPWirePayload struct {
	Phone   string `json:"phone"`
	OTPCode string `json:"otp_code"`
}

// WechatWirePayload 微信登录负载
type WechatWirePayload struct {
	AppID string `json:"app_id"`
	Code  string `json:"code"`
}

// WecomWirePayload 企业微信登录负载
type WecomWirePayload struct {
	CorpID   string `json:"corp_id"`
	AuthCode string `json:"auth_code"`
}

// BuildExplicitWireLoginRequest 将显式 wire payload 转成登录请求。
//
// compatibility 只处理 wire-level 字段名与历史负载兼容；登录方式选择与 payload 校验由 method 层完成。
func BuildExplicitWireLoginRequest(rawMethod string, payload json.RawMessage) (method.LoginRequest, error) {
	authMethod := method.AuthMethod(strings.TrimSpace(rawMethod))
	if !isExplicitWireAuthMethod(authMethod) {
		return method.LoginRequest{}, perrors.WithCode(code.ErrUnsupportedAuthMethod, "unsupported authentication method: %s", rawMethod)
	}
	if len(payload) == 0 {
		return method.LoginRequest{}, perrors.WithCode(code.ErrPayloadInvalid, "method_payload is required")
	}

	switch authMethod {
	case method.AuthMethodPassword:
		return buildPasswordRequest(payload)
	case method.AuthMethodPhoneOTP:
		return buildPhoneOTPRequest(payload)
	case method.AuthMethodWechat:
		return buildWechatRequest(payload)
	case method.AuthMethodWecom:
		return buildWecomRequest(payload)
	default:
		return method.LoginRequest{}, perrors.WithCode(code.ErrUnsupportedAuthMethod, "unsupported authentication method: %s", rawMethod)
	}
}

func isExplicitWireAuthMethod(authMethod method.AuthMethod) bool {
	switch authMethod {
	case method.AuthMethodPassword,
		method.AuthMethodPhoneOTP,
		method.AuthMethodWechat,
		method.AuthMethodWecom:
		return true
	default:
		return false
	}
}

// buildPasswordRequest 构建密码登录请求
func buildPasswordRequest(payload json.RawMessage) (method.LoginRequest, error) {
	var creds PasswordWirePayload
	if err := json.Unmarshal(payload, &creds); err != nil {
		return method.LoginRequest{}, perrors.WithCode(code.ErrBind, "invalid password method_payload: %v", err)
	}
	var tenantID meta.ID
	if creds.TenantID != 0 {
		tenantID = meta.FromUint64(creds.TenantID)
	}
	return method.LoginRequest{
		AuthMethod: method.AuthMethodPassword,
		TenantID:   tenantID,
		Payload: method.PasswordPayload{
			Username: creds.Username,
			Password: creds.Password,
		},
	}, nil
}

// buildPhoneOTPRequest 构建手机号 OTP 登录请求
func buildPhoneOTPRequest(payload json.RawMessage) (method.LoginRequest, error) {
	var creds PhoneOTPWirePayload
	if err := json.Unmarshal(payload, &creds); err != nil {
		return method.LoginRequest{}, perrors.WithCode(code.ErrBind, "invalid phone OTP method_payload: %v", err)
	}
	return method.LoginRequest{
		AuthMethod: method.AuthMethodPhoneOTP,
		Payload: method.PhoneOTPPayload{
			PhoneE164: creds.Phone,
			OTP:       creds.OTPCode,
		},
	}, nil
}

// buildWechatRequest 构建微信登录请求
func buildWechatRequest(payload json.RawMessage) (method.LoginRequest, error) {
	var creds WechatWirePayload
	if err := json.Unmarshal(payload, &creds); err != nil {
		return method.LoginRequest{}, perrors.WithCode(code.ErrBind, "invalid wechat method_payload: %v", err)
	}
	return method.LoginRequest{
		AuthMethod: method.AuthMethodWechat,
		Payload: method.WechatPayload{
			AppID:  creds.AppID,
			JSCode: creds.Code,
		},
	}, nil
}

// buildWecomRequest 构建企业微信登录请求
func buildWecomRequest(payload json.RawMessage) (method.LoginRequest, error) {
	var creds WecomWirePayload
	if err := json.Unmarshal(payload, &creds); err != nil {
		return method.LoginRequest{}, perrors.WithCode(code.ErrBind, "invalid wecom method_payload: %v", err)
	}
	return method.LoginRequest{
		AuthMethod: method.AuthMethodWecom,
		Payload: method.WecomPayload{
			CorpID: creds.CorpID,
			Code:   creds.AuthCode,
		},
	}, nil
}
