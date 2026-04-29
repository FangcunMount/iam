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

// MethodPayload 只在应用层表达协议输入到认证 proof 的映射数据。
type MethodPayload struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string

	Username string
	Password string

	PhoneE164 string
	OTP       string

	WechatAppID  string
	WechatJSCode string

	WecomCorpID string
	WecomCode   string

	BearerToken string
}

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
	payload := MethodPayload{TenantID: req.TenantID}
	var method MethodKind

	if req.Username != nil && req.Password != nil {
		method = MethodPassword
		payload.Username = *req.Username
		payload.Password = *req.Password
		l.Debugw("检测到密码认证",
			"action", logger.ActionLogin,
			"scenario", string(method),
			"username", payload.Username,
		)
	}

	if req.PhoneE164 != nil && req.OTPCode != nil {
		method = MethodPhoneOTP
		payload.PhoneE164 = *req.PhoneE164
		payload.OTP = *req.OTPCode
		l.Debugw("检测到手机OTP认证",
			"action", logger.ActionLogin,
			"scenario", string(method),
			"phone", payload.PhoneE164,
		)
	}

	if req.WechatAppID != nil && req.WechatJSCode != nil {
		method = MethodWechatMini
		payload.WechatAppID = *req.WechatAppID
		payload.WechatJSCode = *req.WechatJSCode
		l.Debugw("检测到微信小程序认证",
			"action", logger.ActionLogin,
			"scenario", string(method),
			"app_id", payload.WechatAppID,
		)
	}

	if req.WecomCorpID != nil && req.WecomCode != nil {
		method = MethodWecom
		payload.WecomCorpID = *req.WecomCorpID
		payload.WecomCode = *req.WecomCode
		l.Debugw("检测到企业微信认证",
			"action", logger.ActionLogin,
			"scenario", string(method),
			"corp_id", payload.WecomCorpID,
		)
	}

	if req.JWTToken != nil {
		method = MethodBearerToken
		payload.BearerToken = *req.JWTToken
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
	payload := MethodPayload{TenantID: req.TenantID}
	switch req.AuthType {
	case AuthTypePassword:
		if req.Username == nil || *req.Username == "" {
			return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "username is required for password authentication")
		}
		if req.Password == nil || *req.Password == "" {
			return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "password is required for password authentication")
		}
		payload.Username = *req.Username
		payload.Password = *req.Password
		return SelectedMethod{Method: MethodPassword, Payload: payload}, nil

	case AuthTypePhoneOTP:
		if req.PhoneE164 == nil || *req.PhoneE164 == "" {
			return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "phone is required for phone otp authentication")
		}
		if req.OTPCode == nil || *req.OTPCode == "" {
			return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "otp_code is required for phone otp authentication")
		}
		payload.PhoneE164 = *req.PhoneE164
		payload.OTP = *req.OTPCode
		return SelectedMethod{Method: MethodPhoneOTP, Payload: payload}, nil

	case AuthTypeWechat:
		if req.WechatAppID == nil || *req.WechatAppID == "" {
			return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "app_id is required for wechat authentication")
		}
		if req.WechatJSCode == nil || *req.WechatJSCode == "" {
			return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "code is required for wechat authentication")
		}
		payload.WechatAppID = *req.WechatAppID
		payload.WechatJSCode = *req.WechatJSCode
		return SelectedMethod{Method: MethodWechatMini, Payload: payload}, nil

	case AuthTypeWecom:
		if req.WecomCorpID == nil || *req.WecomCorpID == "" {
			return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "corp_id is required for wecom authentication")
		}
		if req.WecomCode == nil || *req.WecomCode == "" {
			return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "auth_code is required for wecom authentication")
		}
		payload.WecomCorpID = *req.WecomCorpID
		payload.WecomCode = *req.WecomCode
		return SelectedMethod{Method: MethodWecom, Payload: payload}, nil

	case AuthTypeJWTToken:
		if req.JWTToken == nil || *req.JWTToken == "" {
			return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "bearer token is required for bearer token authentication")
		}
		payload.BearerToken = *req.JWTToken
		return SelectedMethod{Method: MethodBearerToken, Payload: payload}, nil

	default:
		return SelectedMethod{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication method: %s", req.AuthType)
	}
}
