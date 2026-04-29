package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

type MethodKind string

const (
	MethodPassword    MethodKind = "password"
	MethodPhoneOTP    MethodKind = "phone_otp"
	MethodWechatMini  MethodKind = "oauth_wx_minip"
	MethodWecom       MethodKind = "oauth_wecom"
	MethodBearerToken MethodKind = "jwt_token"
)

// SelectedMethod 是应用层完成方法选择后的认证输入。
type SelectedMethod struct {
	Method  MethodKind
	Payload MethodPayload
}

func (m SelectedMethod) TenantID() meta.ID {
	if m.Payload == nil {
		return meta.ZeroID
	}
	return m.Payload.commonPayload().TenantID
}

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

// ScenarioSelector 将登录请求转换为领域认证场景和输入。
type ScenarioSelector interface {
	Select(ctx context.Context, req LoginRequest) (SelectedMethod, error)
}

type defaultScenarioSelector struct {
	legacy   legacyScenarioSelector
	explicit explicitScenarioSelector
}

func newDefaultScenarioSelector() ScenarioSelector {
	return defaultScenarioSelector{}
}

func (s defaultScenarioSelector) Select(ctx context.Context, req LoginRequest) (SelectedMethod, error) {
	if req.SelectionMode == ScenarioSelectionExplicit {
		return s.explicit.Select(ctx, req)
	}
	return s.legacy.Select(ctx, req)
}

type legacyScenarioSelector struct{}

func (s legacyScenarioSelector) Select(ctx context.Context, req LoginRequest) (SelectedMethod, error) {
	l := logger.L(ctx)
	common := methodPayloadCommon{TenantID: req.TenantID}
	var payload MethodPayload
	var method MethodKind

	if req.Username != nil && req.Password != nil {
		method = MethodPassword
		passwordPayload := PasswordPayload{
			methodPayloadCommon: common,
			Username:            *req.Username,
			Password:            *req.Password,
		}
		payload = passwordPayload
		l.Debugw("检测到密码认证",
			"action", logger.ActionLogin,
			"scenario", string(method),
			"username", passwordPayload.Username,
		)
	}

	if req.PhoneE164 != nil && req.OTPCode != nil {
		method = MethodPhoneOTP
		phonePayload := PhoneOTPPayload{
			methodPayloadCommon: common,
			PhoneE164:           *req.PhoneE164,
			OTP:                 *req.OTPCode,
		}
		payload = phonePayload
		l.Debugw("检测到手机OTP认证",
			"action", logger.ActionLogin,
			"scenario", string(method),
			"phone", phonePayload.PhoneE164,
		)
	}

	if req.WechatAppID != nil && req.WechatJSCode != nil {
		method = MethodWechatMini
		wechatPayload := WechatMiniPayload{
			methodPayloadCommon: common,
			AppID:               *req.WechatAppID,
			JSCode:              *req.WechatJSCode,
		}
		payload = wechatPayload
		l.Debugw("检测到微信小程序认证",
			"action", logger.ActionLogin,
			"scenario", string(method),
			"app_id", wechatPayload.AppID,
		)
	}

	if req.WecomCorpID != nil && req.WecomCode != nil {
		method = MethodWecom
		wecomPayload := WecomPayload{
			methodPayloadCommon: common,
			CorpID:              *req.WecomCorpID,
			Code:                *req.WecomCode,
		}
		payload = wecomPayload
		l.Debugw("检测到企业微信认证",
			"action", logger.ActionLogin,
			"scenario", string(method),
			"corp_id", wecomPayload.CorpID,
		)
	}

	if req.JWTToken != nil {
		method = MethodBearerToken
		payload = BearerPayload{
			methodPayloadCommon: common,
			Token:               *req.JWTToken,
		}
		l.Debugw("检测到JWT令牌认证",
			"action", logger.ActionLogin,
			"scenario", string(method),
		)
	}

	if method == "" {
		l.Warnw("未提供有效的认证凭据",
			"action", logger.ActionLogin,
			"result", logger.ResultFailed,
		)
		return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "no valid authentication credentials provided")
	}

	return SelectedMethod{Method: method, Payload: payload}, nil
}

type explicitScenarioSelector struct{}

func (s explicitScenarioSelector) Select(_ context.Context, req LoginRequest) (SelectedMethod, error) {
	common := methodPayloadCommon{TenantID: req.TenantID}
	switch req.AuthType {
	case AuthTypePassword:
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

	case AuthTypePhoneOTP:
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

	case AuthTypeWechat:
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

	case AuthTypeWecom:
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

	case AuthTypeJWTToken:
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

	default:
		return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication method: %s", req.AuthType)
	}
}
