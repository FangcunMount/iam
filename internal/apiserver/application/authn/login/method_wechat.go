package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	idpPort "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

type wechatMethodAuthenticator struct {
	authenticator    *authentication.Authenticator
	wechatAppQuerier idpPort.Repository
	secretVault      idpPort.SecretVault
}

func (a *wechatMethodAuthenticator) Authenticate(ctx context.Context, selected SelectedMethod) (authentication.AuthDecision, error) {
	if a == nil || a.authenticator == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "authenticator is not initialized")
	}
	payload, ok := selected.Payload.(WechatMiniPayload)
	if !ok {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "invalid wechat payload")
	}
	appSecret, err := a.prepareWechatAppSecret(ctx, payload)
	if err != nil {
		return authentication.AuthDecision{}, err
	}
	proof, err := authentication.NewWechatMiniCredential(authentication.WechatMiniProofSpec{
		TenantID:  payload.TenantID,
		RemoteIP:  payload.RemoteIP,
		UserAgent: payload.UserAgent,
		AppID:     payload.AppID,
		AppSecret: appSecret,
		Code:      payload.JSCode,
	})
	if err != nil {
		return authentication.AuthDecision{}, err
	}
	return a.authenticator.Authenticate(ctx, proof)
}

func (a *wechatMethodAuthenticator) prepareWechatAppSecret(ctx context.Context, payload WechatMiniPayload) (string, error) {
	l := logger.L(ctx)
	if a.wechatAppQuerier == nil || a.secretVault == nil {
		l.Errorw("微信应用配置服务不可用",
			"action", logger.ActionLogin,
			"scenario", string(MethodWechatMini),
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "wechat app configuration service not available")
	}

	wechatApp, err := a.wechatAppQuerier.GetByAppID(ctx, payload.AppID)
	if err != nil {
		l.Errorw("查询微信应用配置失败",
			"action", logger.ActionLogin,
			"scenario", string(MethodWechatMini),
			"app_id", payload.AppID,
			"error", err.Error(),
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "failed to query wechat app: %v", err)
	}
	if wechatApp == nil {
		l.Warnw("微信应用不存在",
			"action", logger.ActionLogin,
			"scenario", string(MethodWechatMini),
			"app_id", payload.AppID,
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "wechat app not found: %s", payload.AppID)
	}
	if !wechatApp.IsEnabled() {
		l.Warnw("微信应用已禁用",
			"action", logger.ActionLogin,
			"scenario", string(MethodWechatMini),
			"app_id", payload.AppID,
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "wechat app is disabled: %s", payload.AppID)
	}
	if wechatApp.Cred == nil || wechatApp.Cred.Auth == nil {
		l.Errorw("微信应用凭据缺失",
			"action", logger.ActionLogin,
			"scenario", string(MethodWechatMini),
			"app_id", payload.AppID,
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "wechat app credentials not found")
	}

	appSecretPlain, err := a.secretVault.Decrypt(ctx, wechatApp.Cred.Auth.AppSecretCipher)
	if err != nil {
		l.Errorw("解密应用密钥失败",
			"action", logger.ActionLogin,
			"scenario", string(MethodWechatMini),
			"app_id", payload.AppID,
			"error", err.Error(),
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "failed to decrypt app secret: %v", err)
	}
	return string(appSecretPlain), nil
}
