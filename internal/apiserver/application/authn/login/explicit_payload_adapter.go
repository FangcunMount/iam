package login

import (
	"encoding/json"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type explicitLoginPayloadAdapter interface {
	Build(payload json.RawMessage) (LoginRequest, error)
}

type explicitLoginPayloadAdapterFunc func(payload json.RawMessage) (LoginRequest, error)

func (f explicitLoginPayloadAdapterFunc) Build(payload json.RawMessage) (LoginRequest, error) {
	return f(payload)
}

func explicitLoginPayloadAdapters() map[AuthType]explicitLoginPayloadAdapter {
	return map[AuthType]explicitLoginPayloadAdapter{
		AuthTypePassword: explicitLoginPayloadAdapterFunc(buildPasswordLoginRequest),
		AuthTypePhoneOTP: explicitLoginPayloadAdapterFunc(buildPhoneOTPLoginRequest),
		AuthTypeWechat:   explicitLoginPayloadAdapterFunc(buildWechatLoginRequest),
		AuthTypeWecom:    explicitLoginPayloadAdapterFunc(buildWecomLoginRequest),
	}
}

func buildPasswordLoginRequest(payload json.RawMessage) (LoginRequest, error) {
	var creds PasswordWirePayload
	if err := json.Unmarshal(payload, &creds); err != nil {
		return LoginRequest{}, perrors.WithCode(code.ErrBind, "invalid password method_payload: %v", err)
	}
	loginReq := LoginRequest{
		SelectionMode: SignInSelectionExplicit,
		AuthType:      AuthTypePassword,
		Username:      &creds.Username,
		Password:      &creds.Password,
	}
	if creds.TenantID != 0 {
		loginReq.TenantID = meta.FromUint64(creds.TenantID)
	}
	return loginReq, nil
}

func buildPhoneOTPLoginRequest(payload json.RawMessage) (LoginRequest, error) {
	var creds PhoneOTPWirePayload
	if err := json.Unmarshal(payload, &creds); err != nil {
		return LoginRequest{}, perrors.WithCode(code.ErrBind, "invalid phone OTP method_payload: %v", err)
	}
	return LoginRequest{
		SelectionMode: SignInSelectionExplicit,
		AuthType:      AuthTypePhoneOTP,
		PhoneE164:     &creds.Phone,
		OTPCode:       &creds.OTPCode,
	}, nil
}

func buildWechatLoginRequest(payload json.RawMessage) (LoginRequest, error) {
	var creds WechatWirePayload
	if err := json.Unmarshal(payload, &creds); err != nil {
		return LoginRequest{}, perrors.WithCode(code.ErrBind, "invalid wechat method_payload: %v", err)
	}
	return LoginRequest{
		SelectionMode: SignInSelectionExplicit,
		AuthType:      AuthTypeWechat,
		WechatAppID:   &creds.AppID,
		WechatJSCode:  &creds.Code,
	}, nil
}

func buildWecomLoginRequest(payload json.RawMessage) (LoginRequest, error) {
	var creds WecomWirePayload
	if err := json.Unmarshal(payload, &creds); err != nil {
		return LoginRequest{}, perrors.WithCode(code.ErrBind, "invalid wecom method_payload: %v", err)
	}
	return LoginRequest{
		SelectionMode: SignInSelectionExplicit,
		AuthType:      AuthTypeWecom,
		WecomCorpID:   &creds.CorpID,
		WecomCode:     &creds.AuthCode,
	}, nil
}
