package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	tokenapp "github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	idpPort "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// MethodAuthenticator 执行某一种认证场景。
type MethodAuthenticator interface {
	Authenticate(ctx context.Context, selected SelectedMethod) (authentication.AuthDecision, error)
}

type methodAuthenticatorRouter struct {
	byMethod map[MethodKind]MethodAuthenticator
}

func newMethodAuthenticatorRouter(
	authenticator *authentication.Authenticator,
	tokenVerifier tokenapp.Verifier,
	wechatAppQuerier idpPort.Repository,
	secretVault idpPort.SecretVault,
) *methodAuthenticatorRouter {
	return &methodAuthenticatorRouter{
		byMethod: map[MethodKind]MethodAuthenticator{
			MethodPassword: &domainMethodAuthenticator{
				authenticator: authenticator,
				buildProof:    buildPasswordProof,
			},
			MethodPhoneOTP: &domainMethodAuthenticator{
				authenticator: authenticator,
				buildProof:    buildPhoneOTPProof,
			},
			MethodWechatMini: &wechatMethodAuthenticator{
				authenticator:    authenticator,
				wechatAppQuerier: wechatAppQuerier,
				secretVault:      secretVault,
			},
			MethodWecom: &domainMethodAuthenticator{
				authenticator: authenticator,
				buildProof:    buildWecomProof,
			},
			MethodBearerToken: &bearerMethodAuthenticator{
				tokenVerifier: tokenVerifier,
			},
		},
	}
}

func (r *methodAuthenticatorRouter) Authenticate(ctx context.Context, selected SelectedMethod) (authentication.AuthDecision, error) {
	if r == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "authenticator router is not initialized")
	}
	authenticator := r.byMethod[selected.Method]
	if authenticator == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported authentication scenario: %s", selected.Method)
	}
	return authenticator.Authenticate(ctx, selected)
}

type domainMethodAuthenticator struct {
	authenticator *authentication.Authenticator
	buildProof    func(MethodPayload) (authentication.AuthCredential, error)
}

func (a *domainMethodAuthenticator) Authenticate(ctx context.Context, selected SelectedMethod) (authentication.AuthDecision, error) {
	if a == nil || a.authenticator == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "authenticator is not initialized")
	}
	if a.buildProof == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "credential builder is not initialized")
	}
	proof, err := a.buildProof(selected.Payload)
	if err != nil {
		return authentication.AuthDecision{}, err
	}
	return a.authenticator.Authenticate(ctx, proof)
}

type wechatMethodAuthenticator struct {
	authenticator    *authentication.Authenticator
	wechatAppQuerier idpPort.Repository
	secretVault      idpPort.SecretVault
}

func (a *wechatMethodAuthenticator) Authenticate(ctx context.Context, selected SelectedMethod) (authentication.AuthDecision, error) {
	if a == nil || a.authenticator == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "authenticator is not initialized")
	}
	appSecret, err := a.prepareWechatAppSecret(ctx, selected.Payload)
	if err != nil {
		return authentication.AuthDecision{}, err
	}
	proof, err := authentication.NewWechatMiniCredential(authentication.WechatMiniProofSpec{
		TenantID:  selected.Payload.TenantID,
		RemoteIP:  selected.Payload.RemoteIP,
		UserAgent: selected.Payload.UserAgent,
		AppID:     selected.Payload.WechatAppID,
		AppSecret: appSecret,
		Code:      selected.Payload.WechatJSCode,
	})
	if err != nil {
		return authentication.AuthDecision{}, err
	}
	return a.authenticator.Authenticate(ctx, proof)
}

func (a *wechatMethodAuthenticator) prepareWechatAppSecret(ctx context.Context, payload MethodPayload) (string, error) {
	l := logger.L(ctx)
	if a.wechatAppQuerier == nil || a.secretVault == nil {
		l.Errorw("微信应用配置服务不可用",
			"action", logger.ActionLogin,
			"scenario", string(MethodWechatMini),
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "wechat app configuration service not available")
	}

	wechatApp, err := a.wechatAppQuerier.GetByAppID(ctx, payload.WechatAppID)
	if err != nil {
		l.Errorw("查询微信应用配置失败",
			"action", logger.ActionLogin,
			"scenario", string(MethodWechatMini),
			"app_id", payload.WechatAppID,
			"error", err.Error(),
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "failed to query wechat app: %v", err)
	}
	if wechatApp == nil {
		l.Warnw("微信应用不存在",
			"action", logger.ActionLogin,
			"scenario", string(MethodWechatMini),
			"app_id", payload.WechatAppID,
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "wechat app not found: %s", payload.WechatAppID)
	}
	if !wechatApp.IsEnabled() {
		l.Warnw("微信应用已禁用",
			"action", logger.ActionLogin,
			"scenario", string(MethodWechatMini),
			"app_id", payload.WechatAppID,
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "wechat app is disabled: %s", payload.WechatAppID)
	}
	if wechatApp.Cred == nil || wechatApp.Cred.Auth == nil {
		l.Errorw("微信应用凭据缺失",
			"action", logger.ActionLogin,
			"scenario", string(MethodWechatMini),
			"app_id", payload.WechatAppID,
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "wechat app credentials not found")
	}

	appSecretPlain, err := a.secretVault.Decrypt(ctx, wechatApp.Cred.Auth.AppSecretCipher)
	if err != nil {
		l.Errorw("解密应用密钥失败",
			"action", logger.ActionLogin,
			"scenario", string(MethodWechatMini),
			"app_id", payload.WechatAppID,
			"error", err.Error(),
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "failed to decrypt app secret: %v", err)
	}
	return string(appSecretPlain), nil
}

func buildPasswordProof(payload MethodPayload) (authentication.AuthCredential, error) {
	return authentication.NewPasswordCredential(authentication.PasswordProofSpec{
		TenantID:  payload.TenantID,
		RemoteIP:  payload.RemoteIP,
		UserAgent: payload.UserAgent,
		Username:  payload.Username,
		Password:  payload.Password,
	})
}

func buildPhoneOTPProof(payload MethodPayload) (authentication.AuthCredential, error) {
	return authentication.NewPhoneOTPCredential(authentication.PhoneOTPProofSpec{
		TenantID:  payload.TenantID,
		RemoteIP:  payload.RemoteIP,
		UserAgent: payload.UserAgent,
		PhoneE164: payload.PhoneE164,
		OTP:       payload.OTP,
	})
}

func buildWecomProof(payload MethodPayload) (authentication.AuthCredential, error) {
	return authentication.NewWecomCredential(authentication.WecomProofSpec{
		TenantID: payload.TenantID,
		CorpID:   payload.WecomCorpID,
		Code:     payload.WecomCode,
	})
}

type bearerMethodAuthenticator struct {
	tokenVerifier tokenapp.Verifier
}

func (a *bearerMethodAuthenticator) Authenticate(ctx context.Context, selected SelectedMethod) (authentication.AuthDecision, error) {
	if a == nil || a.tokenVerifier == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "token verifier is not initialized")
	}
	claims, err := a.tokenVerifier.VerifyAccessToken(ctx, selected.Payload.BearerToken)
	if err != nil {
		logger.L(ctx).Warnw("令牌验证失败",
			"action", logger.ActionLogin,
			"scenario", string(MethodBearerToken),
			"error", err.Error(),
		)
		return authentication.AuthDecision{
			OK:      false,
			ErrCode: authentication.ErrInvalidCredential,
		}, nil
	}
	return authentication.AuthDecision{
		OK: true,
		Principal: &authentication.Principal{
			UserID:    claims.UserID,
			AccountID: claims.AccountID,
			TenantID:  claims.TenantID,
			AMR:       []string{"jwt"},
			Claims: map[string]any{
				"auth_method": string(MethodBearerToken),
			},
		},
		CredentialID: meta.FromUint64(0),
	}, nil
}
