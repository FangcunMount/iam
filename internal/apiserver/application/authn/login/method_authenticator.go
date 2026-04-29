package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	idpPort "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// MethodAuthenticator 执行某一种认证场景。
type MethodAuthenticator interface {
	Authenticate(ctx context.Context, selected SelectedScenario) (authentication.AuthDecision, error)
}

type methodAuthenticatorRouter struct {
	byScenario map[authentication.Scenario]MethodAuthenticator
}

func newMethodAuthenticatorRouter(
	authenticater *authentication.Authenticater,
	wechatAppQuerier idpPort.Repository,
	secretVault idpPort.SecretVault,
) *methodAuthenticatorRouter {
	return &methodAuthenticatorRouter{
		byScenario: map[authentication.Scenario]MethodAuthenticator{
			authentication.AuthPassword: &domainMethodAuthenticator{
				scenario:      authentication.AuthPassword,
				authenticater: authenticater,
			},
			authentication.AuthPhoneOTP: &domainMethodAuthenticator{
				scenario:      authentication.AuthPhoneOTP,
				authenticater: authenticater,
			},
			authentication.AuthWxMinip: &wechatMethodAuthenticator{
				authenticater:    authenticater,
				wechatAppQuerier: wechatAppQuerier,
				secretVault:      secretVault,
			},
			authentication.AuthWecom: &domainMethodAuthenticator{
				scenario:      authentication.AuthWecom,
				authenticater: authenticater,
			},
			authentication.AuthBearerToken: &domainMethodAuthenticator{
				scenario:      authentication.AuthBearerToken,
				authenticater: authenticater,
			},
		},
	}
}

func (r *methodAuthenticatorRouter) Authenticate(ctx context.Context, selected SelectedScenario) (authentication.AuthDecision, error) {
	if r == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "authenticator router is not initialized")
	}
	authenticator := r.byScenario[selected.Scenario]
	if authenticator == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication scenario: %s", selected.Scenario)
	}
	return authenticator.Authenticate(ctx, selected)
}

type domainMethodAuthenticator struct {
	scenario      authentication.Scenario
	authenticater *authentication.Authenticater
}

func (a *domainMethodAuthenticator) Authenticate(ctx context.Context, selected SelectedScenario) (authentication.AuthDecision, error) {
	if a == nil || a.authenticater == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "authenticator is not initialized")
	}
	return a.authenticater.Authenticate(ctx, a.scenario, selected.Input)
}

type wechatMethodAuthenticator struct {
	authenticater    *authentication.Authenticater
	wechatAppQuerier idpPort.Repository
	secretVault      idpPort.SecretVault
}

func (a *wechatMethodAuthenticator) Authenticate(ctx context.Context, selected SelectedScenario) (authentication.AuthDecision, error) {
	if a == nil || a.authenticater == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "authenticator is not initialized")
	}
	input, err := a.prepareWechatAppSecret(ctx, selected.Input)
	if err != nil {
		return authentication.AuthDecision{}, err
	}
	return a.authenticater.Authenticate(ctx, authentication.AuthWxMinip, input)
}

func (a *wechatMethodAuthenticator) prepareWechatAppSecret(ctx context.Context, input authentication.AuthInput) (authentication.AuthInput, error) {
	l := logger.L(ctx)
	if a.wechatAppQuerier == nil || a.secretVault == nil {
		l.Errorw("微信应用配置服务不可用",
			"action", logger.ActionLogin,
			"scenario", string(authentication.AuthWxMinip),
		)
		return authentication.AuthInput{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app configuration service not available")
	}

	wechatApp, err := a.wechatAppQuerier.GetByAppID(ctx, input.WxAppID)
	if err != nil {
		l.Errorw("查询微信应用配置失败",
			"action", logger.ActionLogin,
			"scenario", string(authentication.AuthWxMinip),
			"app_id", input.WxAppID,
			"error", err.Error(),
		)
		return authentication.AuthInput{}, perrors.WithCode(code.ErrInvalidArgument, "failed to query wechat app: %v", err)
	}
	if wechatApp == nil {
		l.Warnw("微信应用不存在",
			"action", logger.ActionLogin,
			"scenario", string(authentication.AuthWxMinip),
			"app_id", input.WxAppID,
		)
		return authentication.AuthInput{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app not found: %s", input.WxAppID)
	}
	if !wechatApp.IsEnabled() {
		l.Warnw("微信应用已禁用",
			"action", logger.ActionLogin,
			"scenario", string(authentication.AuthWxMinip),
			"app_id", input.WxAppID,
		)
		return authentication.AuthInput{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app is disabled: %s", input.WxAppID)
	}
	if wechatApp.Cred == nil || wechatApp.Cred.Auth == nil {
		l.Errorw("微信应用凭据缺失",
			"action", logger.ActionLogin,
			"scenario", string(authentication.AuthWxMinip),
			"app_id", input.WxAppID,
		)
		return authentication.AuthInput{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app credentials not found")
	}

	appSecretPlain, err := a.secretVault.Decrypt(ctx, wechatApp.Cred.Auth.AppSecretCipher)
	if err != nil {
		l.Errorw("解密应用密钥失败",
			"action", logger.ActionLogin,
			"scenario", string(authentication.AuthWxMinip),
			"app_id", input.WxAppID,
			"error", err.Error(),
		)
		return authentication.AuthInput{}, perrors.WithCode(code.ErrInvalidArgument, "failed to decrypt app secret: %v", err)
	}
	input.WxAppSecret = string(appSecretPlain)
	return input, nil
}
