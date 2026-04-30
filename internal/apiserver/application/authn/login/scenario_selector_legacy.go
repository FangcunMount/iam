package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"

	"github.com/FangcunMount/iam/internal/pkg/code"
)

type legacyScenarioSelector struct{}

func (s legacyScenarioSelector) Select(ctx context.Context, req LoginRequest) (SelectedMethod, error) {
	l := logger.L(ctx)
	common := commonPayloadFromRequest(req)
	var selected SelectedMethod

	if payload, ok := legacyPasswordPayload(req, common); ok {
		selected = SelectedMethod{Method: MethodPassword, Payload: payload}
		l.Debugw("检测到密码认证",
			"action", logger.ActionLogin,
			"scenario", string(selected.Method),
			"username", payload.Username,
		)
	}

	if payload, ok := legacyPhoneOTPPayload(req, common); ok {
		selected = SelectedMethod{Method: MethodPhoneOTP, Payload: payload}
		l.Debugw("检测到手机OTP认证",
			"action", logger.ActionLogin,
			"scenario", string(selected.Method),
			"phone", payload.PhoneE164,
		)
	}

	if payload, ok := legacyWechatMiniPayload(req, common); ok {
		selected = SelectedMethod{Method: MethodWechatMini, Payload: payload}
		l.Debugw("检测到微信小程序认证",
			"action", logger.ActionLogin,
			"scenario", string(selected.Method),
			"app_id", payload.AppID,
		)
	}

	if payload, ok := legacyWecomPayload(req, common); ok {
		selected = SelectedMethod{Method: MethodWecom, Payload: payload}
		l.Debugw("检测到企业微信认证",
			"action", logger.ActionLogin,
			"scenario", string(selected.Method),
			"corp_id", payload.CorpID,
		)
	}

	if payload, ok := legacyBearerPayload(req, common); ok {
		selected = SelectedMethod{Method: MethodBearerToken, Payload: payload}
		l.Debugw("检测到JWT令牌认证",
			"action", logger.ActionLogin,
			"scenario", string(selected.Method),
		)
	}

	if selected.Method == "" {
		l.Warnw("未提供有效的认证凭据",
			"action", logger.ActionLogin,
			"result", logger.ResultFailed,
		)
		return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "no valid authentication credentials provided")
	}

	return selected, nil
}

func legacyPasswordPayload(req LoginRequest, common methodPayloadCommon) (PasswordPayload, bool) {
	if req.Username == nil || req.Password == nil {
		return PasswordPayload{}, false
	}
	return PasswordPayload{
		methodPayloadCommon: common,
		Username:            *req.Username,
		Password:            *req.Password,
	}, true
}

func legacyPhoneOTPPayload(req LoginRequest, common methodPayloadCommon) (PhoneOTPPayload, bool) {
	if req.PhoneE164 == nil || req.OTPCode == nil {
		return PhoneOTPPayload{}, false
	}
	return PhoneOTPPayload{
		methodPayloadCommon: common,
		PhoneE164:           *req.PhoneE164,
		OTP:                 *req.OTPCode,
	}, true
}

func legacyWechatMiniPayload(req LoginRequest, common methodPayloadCommon) (WechatMiniPayload, bool) {
	if req.WechatAppID == nil || req.WechatJSCode == nil {
		return WechatMiniPayload{}, false
	}
	return WechatMiniPayload{
		methodPayloadCommon: common,
		AppID:               *req.WechatAppID,
		JSCode:              *req.WechatJSCode,
	}, true
}

func legacyWecomPayload(req LoginRequest, common methodPayloadCommon) (WecomPayload, bool) {
	if req.WecomCorpID == nil || req.WecomCode == nil {
		return WecomPayload{}, false
	}
	return WecomPayload{
		methodPayloadCommon: common,
		CorpID:              *req.WecomCorpID,
		Code:                *req.WecomCode,
	}, true
}

func legacyBearerPayload(req LoginRequest, common methodPayloadCommon) (BearerPayload, bool) {
	if req.JWTToken == nil {
		return BearerPayload{}, false
	}
	return BearerPayload{
		methodPayloadCommon: common,
		Token:               *req.JWTToken,
	}, true
}
