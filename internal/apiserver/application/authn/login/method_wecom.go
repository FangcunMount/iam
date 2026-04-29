package login

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	idpPort "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

type wecomMethodAuthenticator struct {
	authenticator    *authentication.Authenticator
	wechatAppQuerier idpPort.Repository
	secretVault      idpPort.SecretVault
	config           WecomConfig
}

func (a *wecomMethodAuthenticator) Authenticate(ctx context.Context, selected SelectedMethod) (authentication.AuthDecision, error) {
	if a == nil || a.authenticator == nil {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "authenticator is not initialized")
	}
	payload, ok := selected.Payload.(WecomPayload)
	if !ok {
		return authentication.AuthDecision{}, perrors.WithCode(code.ErrInvalidArgument, "invalid wecom payload")
	}
	appConfig, err := a.prepareWecomAppConfig(ctx, payload)
	if err != nil {
		return authentication.AuthDecision{}, err
	}
	proof, err := authentication.NewWecomCredential(authentication.WecomProofSpec{
		TenantID:   payload.TenantID,
		RemoteIP:   payload.RemoteIP,
		UserAgent:  payload.UserAgent,
		CorpID:     payload.CorpID,
		AgentID:    appConfig.agentID,
		CorpSecret: appConfig.corpSecret,
		Code:       payload.Code,
	})
	if err != nil {
		return authentication.AuthDecision{}, err
	}
	return a.authenticator.Authenticate(ctx, proof)
}

type wecomAppConfig struct {
	agentID    string
	corpSecret string
}

func (a *wecomMethodAuthenticator) prepareWecomAppConfig(ctx context.Context, payload WecomPayload) (wecomAppConfig, error) {
	l := logger.L(ctx)
	if a.wechatAppQuerier == nil || a.secretVault == nil {
		l.Errorw("企业微信应用配置服务不可用",
			"action", logger.ActionLogin,
			"scenario", string(MethodWecom),
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrInvalidArgument, "wecom app configuration service not available")
	}

	agentID := strings.TrimSpace(a.config.AgentID)
	if agentID == "" {
		l.Errorw("企业微信应用 agent_id 未配置",
			"action", logger.ActionLogin,
			"scenario", string(MethodWecom),
			"corp_id", payload.CorpID,
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrInvalidArgument, "wecom agent_id is required in server configuration")
	}

	wechatApp, err := a.wechatAppQuerier.GetByAppID(ctx, payload.CorpID)
	if err != nil {
		l.Errorw("查询企业微信应用配置失败",
			"action", logger.ActionLogin,
			"scenario", string(MethodWecom),
			"corp_id", payload.CorpID,
			"error", err.Error(),
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrInvalidArgument, "failed to query wecom app: %v", err)
	}
	if wechatApp == nil {
		l.Warnw("企业微信应用不存在",
			"action", logger.ActionLogin,
			"scenario", string(MethodWecom),
			"corp_id", payload.CorpID,
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrInvalidArgument, "wecom app not found: %s", payload.CorpID)
	}
	if !wechatApp.IsEnabled() {
		l.Warnw("企业微信应用已禁用",
			"action", logger.ActionLogin,
			"scenario", string(MethodWecom),
			"corp_id", payload.CorpID,
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrInvalidArgument, "wecom app is disabled: %s", payload.CorpID)
	}
	if wechatApp.Cred == nil || wechatApp.Cred.Auth == nil {
		l.Errorw("企业微信应用凭据缺失",
			"action", logger.ActionLogin,
			"scenario", string(MethodWecom),
			"corp_id", payload.CorpID,
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrInvalidArgument, "wecom app credentials not found")
	}

	corpSecretPlain, err := a.secretVault.Decrypt(ctx, wechatApp.Cred.Auth.AppSecretCipher)
	if err != nil {
		l.Errorw("解密企业微信应用密钥失败",
			"action", logger.ActionLogin,
			"scenario", string(MethodWecom),
			"corp_id", payload.CorpID,
			"error", err.Error(),
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrInvalidArgument, "failed to decrypt wecom corp secret: %v", err)
	}
	return wecomAppConfig{agentID: agentID, corpSecret: string(corpSecretPlain)}, nil
}
