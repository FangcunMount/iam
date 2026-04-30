package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"

	"github.com/FangcunMount/iam/internal/pkg/code"
)

type explicitScenarioSelector struct{}

func (s explicitScenarioSelector) Select(_ context.Context, req LoginRequest) (SelectedMethod, error) {
	common := commonPayloadFromRequest(req)
	switch req.AuthType {
	case AuthTypePassword:
		return explicitPasswordMethod(req, common)

	case AuthTypePhoneOTP:
		return explicitPhoneOTPMethod(req, common)

	case AuthTypeWechat:
		return explicitWechatMiniMethod(req, common)

	case AuthTypeWecom:
		return explicitWecomMethod(req, common)

	case AuthTypeJWTToken:
		return explicitBearerMethod(req, common)

	default:
		return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication method: %s", req.AuthType)
	}
}

func explicitPasswordMethod(req LoginRequest, common methodPayloadCommon) (SelectedMethod, error) {
	if req.Username == nil || *req.Username == "" {
		return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "username is required for password authentication")
	}
	if req.Password == nil || *req.Password == "" {
		return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "password is required for password authentication")
	}
	return SelectedMethod{
		Method: MethodPassword,
		Payload: PasswordPayload{
			methodPayloadCommon: common,
			Username:            *req.Username,
			Password:            *req.Password,
		},
	}, nil
}

func explicitPhoneOTPMethod(req LoginRequest, common methodPayloadCommon) (SelectedMethod, error) {
	if req.PhoneE164 == nil || *req.PhoneE164 == "" {
		return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "phone is required for phone otp authentication")
	}
	if req.OTPCode == nil || *req.OTPCode == "" {
		return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "otp_code is required for phone otp authentication")
	}
	return SelectedMethod{
		Method: MethodPhoneOTP,
		Payload: PhoneOTPPayload{
			methodPayloadCommon: common,
			PhoneE164:           *req.PhoneE164,
			OTP:                 *req.OTPCode,
		},
	}, nil
}

func explicitWechatMiniMethod(req LoginRequest, common methodPayloadCommon) (SelectedMethod, error) {
	if req.WechatAppID == nil || *req.WechatAppID == "" {
		return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "app_id is required for wechat authentication")
	}
	if req.WechatJSCode == nil || *req.WechatJSCode == "" {
		return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "code is required for wechat authentication")
	}
	return SelectedMethod{
		Method: MethodWechatMini,
		Payload: WechatMiniPayload{
			methodPayloadCommon: common,
			AppID:               *req.WechatAppID,
			JSCode:              *req.WechatJSCode,
		},
	}, nil
}

func explicitWecomMethod(req LoginRequest, common methodPayloadCommon) (SelectedMethod, error) {
	if req.WecomCorpID == nil || *req.WecomCorpID == "" {
		return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "corp_id is required for wecom authentication")
	}
	if req.WecomCode == nil || *req.WecomCode == "" {
		return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "auth_code is required for wecom authentication")
	}
	return SelectedMethod{
		Method: MethodWecom,
		Payload: WecomPayload{
			methodPayloadCommon: common,
			CorpID:              *req.WecomCorpID,
			Code:                *req.WecomCode,
		},
	}, nil
}

func explicitBearerMethod(req LoginRequest, common methodPayloadCommon) (SelectedMethod, error) {
	if req.JWTToken == nil || *req.JWTToken == "" {
		return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "bearer token is required for bearer token authentication")
	}
	return SelectedMethod{
		Method: MethodBearerToken,
		Payload: BearerPayload{
			methodPayloadCommon: common,
			Token:               *req.JWTToken,
		},
	}, nil
}
