package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// SelectedScenario 是应用层完成场景选择后的认证输入。
type SelectedScenario struct {
	Scenario authentication.Scenario
	Input    authentication.AuthInput
}

// ScenarioSelector 将登录请求转换为领域认证场景和输入。
type ScenarioSelector interface {
	Select(ctx context.Context, req LoginRequest) (SelectedScenario, error)
}

type defaultScenarioSelector struct {
	legacy   legacyScenarioSelector
	explicit explicitScenarioSelector
}

func newDefaultScenarioSelector() ScenarioSelector {
	return defaultScenarioSelector{}
}

func (s defaultScenarioSelector) Select(ctx context.Context, req LoginRequest) (SelectedScenario, error) {
	if req.SelectionMode == ScenarioSelectionExplicit {
		return s.explicit.Select(ctx, req)
	}
	return s.legacy.Select(ctx, req)
}

type legacyScenarioSelector struct{}

func (s legacyScenarioSelector) Select(ctx context.Context, req LoginRequest) (SelectedScenario, error) {
	l := logger.L(ctx)
	input := authentication.AuthInput{TenantID: req.TenantID}
	var scenario authentication.Scenario

	if req.Username != nil && req.Password != nil {
		scenario = authentication.AuthPassword
		input.Username = *req.Username
		input.Password = *req.Password
		l.Debugw("检测到密码认证",
			"action", logger.ActionLogin,
			"scenario", string(scenario),
			"username", input.Username,
		)
	}

	if req.PhoneE164 != nil && req.OTPCode != nil {
		scenario = authentication.AuthPhoneOTP
		input.PhoneE164 = *req.PhoneE164
		input.OTP = *req.OTPCode
		l.Debugw("检测到手机OTP认证",
			"action", logger.ActionLogin,
			"scenario", string(scenario),
			"phone", input.PhoneE164,
		)
	}

	if req.WechatAppID != nil && req.WechatJSCode != nil {
		scenario = authentication.AuthWxMinip
		input.WxAppID = *req.WechatAppID
		input.WxJsCode = *req.WechatJSCode
		l.Debugw("检测到微信小程序认证",
			"action", logger.ActionLogin,
			"scenario", string(scenario),
			"app_id", input.WxAppID,
		)
	}

	if req.WecomCorpID != nil && req.WecomCode != nil {
		scenario = authentication.AuthWecom
		input.WecomCorpID = *req.WecomCorpID
		input.WecomCode = *req.WecomCode
		l.Debugw("检测到企业微信认证",
			"action", logger.ActionLogin,
			"scenario", string(scenario),
			"corp_id", input.WecomCorpID,
		)
	}

	if req.JWTToken != nil {
		scenario = authentication.AuthJWTToken
		input.AccessToken = *req.JWTToken
		l.Debugw("检测到JWT令牌认证",
			"action", logger.ActionLogin,
			"scenario", string(scenario),
		)
	}

	if scenario == "" {
		l.Warnw("未提供有效的认证凭据",
			"action", logger.ActionLogin,
			"result", logger.ResultFailed,
		)
		return SelectedScenario{}, perrors.WithCode(code.ErrInvalidArgument, "no valid authentication credentials provided")
	}

	return SelectedScenario{Scenario: scenario, Input: input}, nil
}

type explicitScenarioSelector struct{}

func (s explicitScenarioSelector) Select(_ context.Context, req LoginRequest) (SelectedScenario, error) {
	input := authentication.AuthInput{TenantID: req.TenantID}
	switch req.AuthType {
	case AuthTypePassword:
		if req.Username == nil || *req.Username == "" {
			return SelectedScenario{}, perrors.WithCode(code.ErrInvalidArgument, "username is required for password authentication")
		}
		if req.Password == nil || *req.Password == "" {
			return SelectedScenario{}, perrors.WithCode(code.ErrInvalidArgument, "password is required for password authentication")
		}
		input.Username = *req.Username
		input.Password = *req.Password
		return SelectedScenario{Scenario: authentication.AuthPassword, Input: input}, nil

	case AuthTypePhoneOTP:
		if req.PhoneE164 == nil || *req.PhoneE164 == "" {
			return SelectedScenario{}, perrors.WithCode(code.ErrInvalidArgument, "phone is required for phone otp authentication")
		}
		if req.OTPCode == nil || *req.OTPCode == "" {
			return SelectedScenario{}, perrors.WithCode(code.ErrInvalidArgument, "otp_code is required for phone otp authentication")
		}
		input.PhoneE164 = *req.PhoneE164
		input.OTP = *req.OTPCode
		return SelectedScenario{Scenario: authentication.AuthPhoneOTP, Input: input}, nil

	case AuthTypeWechat:
		if req.WechatAppID == nil || *req.WechatAppID == "" {
			return SelectedScenario{}, perrors.WithCode(code.ErrInvalidArgument, "app_id is required for wechat authentication")
		}
		if req.WechatJSCode == nil || *req.WechatJSCode == "" {
			return SelectedScenario{}, perrors.WithCode(code.ErrInvalidArgument, "code is required for wechat authentication")
		}
		input.WxAppID = *req.WechatAppID
		input.WxJsCode = *req.WechatJSCode
		return SelectedScenario{Scenario: authentication.AuthWxMinip, Input: input}, nil

	case AuthTypeWecom:
		if req.WecomCorpID == nil || *req.WecomCorpID == "" {
			return SelectedScenario{}, perrors.WithCode(code.ErrInvalidArgument, "corp_id is required for wecom authentication")
		}
		if req.WecomCode == nil || *req.WecomCode == "" {
			return SelectedScenario{}, perrors.WithCode(code.ErrInvalidArgument, "auth_code is required for wecom authentication")
		}
		input.WecomCorpID = *req.WecomCorpID
		input.WecomCode = *req.WecomCode
		return SelectedScenario{Scenario: authentication.AuthWecom, Input: input}, nil

	case AuthTypeJWTToken:
		if req.JWTToken == nil || *req.JWTToken == "" {
			return SelectedScenario{}, perrors.WithCode(code.ErrInvalidArgument, "jwt token is required for jwt token authentication")
		}
		input.AccessToken = *req.JWTToken
		return SelectedScenario{Scenario: authentication.AuthJWTToken, Input: input}, nil

	default:
		return SelectedScenario{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication method: %s", req.AuthType)
	}
}
