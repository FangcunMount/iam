package login

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	credDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/credential"
	idpPort "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

type wecomAdapter struct {
	wechatAppQuerier idpPort.Repository
	secretVault      idpPort.SecretVault
	config           WecomConfig
}

func newWecomAdapter(repo idpPort.Repository, vault idpPort.SecretVault, config WecomConfig) *wecomAdapter {
	return &wecomAdapter{
		wechatAppQuerier: repo,
		secretVault:      vault,
		config:           config,
	}
}

func (*wecomAdapter) Kind() SignInKind {
	return SignInKind(credDomain.CredOAuthWecom)
}

func (*wecomAdapter) AuthType() AuthType {
	return AuthTypeWecom
}

func (*wecomAdapter) TryLegacy(req SignInCommand, common methodPayloadCommon) (MethodPayload, bool) {
	if req.WecomCorpID == nil || req.WecomCode == nil {
		return nil, false
	}
	return WecomPayload{
		methodPayloadCommon: common,
		CorpID:              *req.WecomCorpID,
		Code:                *req.WecomCode,
	}, true
}

func (*wecomAdapter) BuildExplicit(req SignInCommand, common methodPayloadCommon) (MethodPayload, error) {
	if req.WecomCorpID == nil || *req.WecomCorpID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "corp_id is required for wecom authentication")
	}
	if req.WecomCode == nil || *req.WecomCode == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "auth_code is required for wecom authentication")
	}
	return WecomPayload{
		methodPayloadCommon: common,
		CorpID:              *req.WecomCorpID,
		Code:                *req.WecomCode,
	}, nil
}

func (a *wecomAdapter) PrepareProof(ctx context.Context, payload MethodPayload) (authentication.AuthCredential, error) {
	wecomPayload, ok := payload.(WecomPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid wecom payload")
	}
	appConfig, err := a.prepareWecomAppConfig(ctx, wecomPayload)
	if err != nil {
		return nil, err
	}
	return authentication.NewWecomCredential(authentication.WecomProofSpec{
		TenantID:   wecomPayload.TenantID,
		RemoteIP:   wecomPayload.RemoteIP,
		UserAgent:  wecomPayload.UserAgent,
		CorpID:     wecomPayload.CorpID,
		AgentID:    appConfig.agentID,
		CorpSecret: appConfig.corpSecret,
		Code:       wecomPayload.Code,
	})
}

type wecomAppConfig struct {
	agentID    string
	corpSecret string
}

func (a *wecomAdapter) prepareWecomAppConfig(ctx context.Context, payload WecomPayload) (wecomAppConfig, error) {
	l := logger.L(ctx)
	if a.wechatAppQuerier == nil || a.secretVault == nil {
		l.Errorw("企业微信应用配置服务不可用",
			"action", logger.ActionLogin,
			"credential_type", string(credDomain.CredOAuthWecom),
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrInvalidArgument, "wecom app configuration service not available")
	}

	agentID := strings.TrimSpace(a.config.AgentID)
	if agentID == "" {
		l.Errorw("企业微信应用 agent_id 未配置",
			"action", logger.ActionLogin,
			"credential_type", string(credDomain.CredOAuthWecom),
			"corp_id", payload.CorpID,
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrInvalidArgument, "wecom agent_id is required in server configuration")
	}

	wechatApp, err := a.wechatAppQuerier.GetByAppID(ctx, payload.CorpID)
	if err != nil {
		l.Errorw("查询企业微信应用配置失败",
			"action", logger.ActionLogin,
			"credential_type", string(credDomain.CredOAuthWecom),
			"corp_id", payload.CorpID,
			"error", err.Error(),
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrInvalidArgument, "failed to query wecom app: %v", err)
	}
	if wechatApp == nil {
		l.Warnw("企业微信应用不存在",
			"action", logger.ActionLogin,
			"credential_type", string(credDomain.CredOAuthWecom),
			"corp_id", payload.CorpID,
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrInvalidArgument, "wecom app not found: %s", payload.CorpID)
	}
	if !wechatApp.IsEnabled() {
		l.Warnw("企业微信应用已禁用",
			"action", logger.ActionLogin,
			"credential_type", string(credDomain.CredOAuthWecom),
			"corp_id", payload.CorpID,
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrInvalidArgument, "wecom app is disabled: %s", payload.CorpID)
	}
	if wechatApp.Cred == nil || wechatApp.Cred.Auth == nil {
		l.Errorw("企业微信应用凭据缺失",
			"action", logger.ActionLogin,
			"credential_type", string(credDomain.CredOAuthWecom),
			"corp_id", payload.CorpID,
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrInvalidArgument, "wecom app credentials not found")
	}

	corpSecretPlain, err := a.secretVault.Decrypt(ctx, wechatApp.Cred.Auth.AppSecretCipher)
	if err != nil {
		l.Errorw("解密企业微信应用密钥失败",
			"action", logger.ActionLogin,
			"credential_type", string(credDomain.CredOAuthWecom),
			"corp_id", payload.CorpID,
			"error", err.Error(),
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrInvalidArgument, "failed to decrypt wecom corp secret: %v", err)
	}
	return wecomAppConfig{agentID: agentID, corpSecret: string(corpSecretPlain)}, nil
}
