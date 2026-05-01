package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	idpPort "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

type wechatMiniAdapter struct {
	wechatAppQuerier idpPort.Repository
	secretVault      idpPort.SecretVault
}

func newWechatMiniAdapter(repo idpPort.Repository, vault idpPort.SecretVault) *wechatMiniAdapter {
	return &wechatMiniAdapter{
		wechatAppQuerier: repo,
		secretVault:      vault,
	}
}

func (*wechatMiniAdapter) Kind() SignInKind {
	return SignInKind(credDomain.CredOAuthWxMinip)
}

func (*wechatMiniAdapter) AuthType() AuthType {
	return AuthTypeWechat
}

func (*wechatMiniAdapter) TryLegacy(req SignInCommand, common methodPayloadCommon) (MethodPayload, bool) {
	if req.WechatAppID == nil || req.WechatJSCode == nil {
		return nil, false
	}
	return WechatMiniPayload{
		methodPayloadCommon: common,
		AppID:               *req.WechatAppID,
		JSCode:              *req.WechatJSCode,
	}, true
}

func (*wechatMiniAdapter) BuildExplicit(req SignInCommand, common methodPayloadCommon) (MethodPayload, error) {
	if req.WechatAppID == nil || *req.WechatAppID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "app_id is required for wechat authentication")
	}
	if req.WechatJSCode == nil || *req.WechatJSCode == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "code is required for wechat authentication")
	}
	return WechatMiniPayload{
		methodPayloadCommon: common,
		AppID:               *req.WechatAppID,
		JSCode:              *req.WechatJSCode,
	}, nil
}

func (a *wechatMiniAdapter) PrepareProof(ctx context.Context, payload MethodPayload) (authentication.AuthCredential, error) {
	wechatPayload, ok := payload.(WechatMiniPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid wechat payload")
	}
	appSecret, err := a.prepareWechatAppSecret(ctx, wechatPayload)
	if err != nil {
		return nil, err
	}
	return authentication.NewWechatMiniCredential(authentication.WechatMiniProofSpec{
		TenantID:  wechatPayload.TenantID,
		RemoteIP:  wechatPayload.RemoteIP,
		UserAgent: wechatPayload.UserAgent,
		AppID:     wechatPayload.AppID,
		AppSecret: appSecret,
		Code:      wechatPayload.JSCode,
	})
}

func (a *wechatMiniAdapter) prepareWechatAppSecret(ctx context.Context, payload WechatMiniPayload) (string, error) {
	l := logger.L(ctx)
	if a.wechatAppQuerier == nil || a.secretVault == nil {
		l.Errorw("微信应用配置服务不可用",
			"action", logger.ActionLogin,
			"credential_type", string(credDomain.CredOAuthWxMinip),
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "wechat app configuration service not available")
	}

	wechatApp, err := a.wechatAppQuerier.GetByAppID(ctx, payload.AppID)
	if err != nil {
		l.Errorw("查询微信应用配置失败",
			"action", logger.ActionLogin,
			"credential_type", string(credDomain.CredOAuthWxMinip),
			"app_id", payload.AppID,
			"error", err.Error(),
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "failed to query wechat app: %v", err)
	}
	if wechatApp == nil {
		l.Warnw("微信应用不存在",
			"action", logger.ActionLogin,
			"credential_type", string(credDomain.CredOAuthWxMinip),
			"app_id", payload.AppID,
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "wechat app not found: %s", payload.AppID)
	}
	if !wechatApp.IsEnabled() {
		l.Warnw("微信应用已禁用",
			"action", logger.ActionLogin,
			"credential_type", string(credDomain.CredOAuthWxMinip),
			"app_id", payload.AppID,
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "wechat app is disabled: %s", payload.AppID)
	}
	if wechatApp.Cred == nil || wechatApp.Cred.Auth == nil {
		l.Errorw("微信应用凭据缺失",
			"action", logger.ActionLogin,
			"credential_type", string(credDomain.CredOAuthWxMinip),
			"app_id", payload.AppID,
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "wechat app credentials not found")
	}

	appSecretPlain, err := a.secretVault.Decrypt(ctx, wechatApp.Cred.Auth.AppSecretCipher)
	if err != nil {
		l.Errorw("解密应用密钥失败",
			"action", logger.ActionLogin,
			"credential_type", string(credDomain.CredOAuthWxMinip),
			"app_id", payload.AppID,
			"error", err.Error(),
		)
		return "", perrors.WithCode(code.ErrInvalidArgument, "failed to decrypt app secret: %v", err)
	}
	return string(appSecretPlain), nil
}
