// Package loginv2 provides the REST AuthN v2 explicit login client.
package loginv2

import (
	"encoding/json"
	"reflect"

	sdkerrors "github.com/FangcunMount/iam/v3/pkg/sdk/errors"
	"google.golang.org/grpc/codes"
)

// AuthMethod is the explicit REST AuthN v2 login method.
type AuthMethod string

const (
	AuthMethodPassword   AuthMethod = "password"
	AuthMethodPhoneOTP   AuthMethod = "phone_otp"
	AuthMethodWechat     AuthMethod = "wechat"
	AuthMethodWechatScan AuthMethod = "wechat_scan"
	AuthMethodWecom      AuthMethod = "wecom"
)

// LoginRequest is the REST AuthN v2 login request.
type LoginRequest struct {
	AuthMethod    AuthMethod `json:"auth_method"`
	MethodPayload any        `json:"method_payload"`
	DeviceID      string     `json:"device_id,omitempty"`
}

// Validate checks only the public REST v2 contract boundary.
func (r LoginRequest) Validate() error {
	switch r.AuthMethod {
	case AuthMethodPassword, AuthMethodPhoneOTP, AuthMethodWechat, AuthMethodWechatScan, AuthMethodWecom:
	default:
		return invalidArgument("invalid authentication method")
	}
	if isNilPayload(r.MethodPayload) {
		return invalidArgument("method_payload is required")
	}
	return nil
}

// PasswordPayload is the password login payload.
type PasswordPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TenantID uint64 `json:"tenant_id,omitempty"`
}

// PhoneOTPPayload is the phone OTP login payload.
type PhoneOTPPayload struct {
	Phone   string `json:"phone"`
	OTPCode string `json:"otp_code"`
}

// WechatPayload is the WeChat mini program login payload.
type WechatPayload struct {
	AppID string `json:"app_id"`
	Code  string `json:"code"`
}

// WechatScanPayload is the WeChat Open Platform website QR login payload.
type WechatScanPayload struct {
	AppID string `json:"app_id"`
	Code  string `json:"code"`
	State string `json:"state"`
}

// WecomPayload is the WeCom login payload.
type WecomPayload struct {
	CorpID   string `json:"corp_id"`
	AuthCode string `json:"auth_code"`
}

// TokenPair is the successful login token pair.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

func isNilPayload(payload any) bool {
	if payload == nil {
		return true
	}
	switch value := payload.(type) {
	case json.RawMessage:
		return len(value) == 0
	case *json.RawMessage:
		return value == nil || len(*value) == 0
	}
	value := reflect.ValueOf(payload)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func invalidArgument(message string) error {
	return &sdkerrors.IAMError{
		Code:     codes.InvalidArgument.String(),
		Message:  message,
		GRPCCode: codes.InvalidArgument,
	}
}
