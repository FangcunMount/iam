package proof

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/method"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	idpPort "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// wechatBuilder 微信小程序登录方式构造器
type wechatBuilder struct {
	wechatAppQuerier idpPort.Repository
	secretVault      idpPort.SecretVault
}

// NewWechatBuilder 创建微信小程序登录方式构造器
func newWechatBuilder(repo idpPort.Repository, vault idpPort.SecretVault) Builder {
	return &wechatBuilder{wechatAppQuerier: repo, secretVault: vault}
}

// CredentialKind 返回认证证明类型
func (*wechatBuilder) CredentialKind() method.CredentialKind {
	return method.CredentialKindWechatMinip
}

// Build 构建微信小程序登录方式
func (b *wechatBuilder) Build(ctx context.Context, payload method.Payload, common method.CommonPayload) (authentication.AuthCredential, error) {
	wechatPayload, ok := payload.(method.WechatPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrProofBuildFailed, "invalid wechat payload")
	}
	// 准备微信小程序应用密钥
	appSecret, err := b.prepareWechatAppSecret(ctx, wechatPayload)
	if err != nil {
		return nil, err
	}

	// 构建微信小程序认证凭据
	return authentication.NewWechatMiniCredential(authentication.WechatMiniProofSpec{
		TenantID:  common.TenantID,
		RemoteIP:  common.RemoteIP,
		UserAgent: common.UserAgent,
		AppID:     wechatPayload.AppID,
		AppSecret: appSecret,
		Code:      wechatPayload.JSCode,
	})
}

// prepareWechatAppSecret 准备微信小程序应用密钥
func (b *wechatBuilder) prepareWechatAppSecret(ctx context.Context, payload method.WechatPayload) (string, error) {
	l := logger.L(ctx)
	if b.wechatAppQuerier == nil || b.secretVault == nil {
		l.Errorw("微信应用配置服务不可用",
			"action", logger.ActionLogin,
			"credential_kind", string(method.CredentialKindWechatMinip),
		)
		return "", perrors.WithCode(code.ErrProofBuildFailed, "wechat app configuration service not available")
	}

	wechatApp, err := b.wechatAppQuerier.GetByAppID(ctx, payload.AppID)
	if err != nil {
		l.Errorw("查询微信应用配置失败",
			"action", logger.ActionLogin,
			"credential_kind", string(method.CredentialKindWechatMinip),
			"app_id", payload.AppID,
			"error", err.Error(),
		)
		return "", perrors.WithCode(code.ErrProofBuildFailed, "failed to query wechat app: %v", err)
	}
	if wechatApp == nil {
		l.Warnw("微信应用不存在",
			"action", logger.ActionLogin,
			"credential_kind", string(method.CredentialKindWechatMinip),
			"app_id", payload.AppID,
		)
		return "", perrors.WithCode(code.ErrProofBuildFailed, "wechat app not found: %s", payload.AppID)
	}
	if !wechatApp.IsEnabled() {
		l.Warnw("微信应用已禁用",
			"action", logger.ActionLogin,
			"credential_kind", string(method.CredentialKindWechatMinip),
			"app_id", payload.AppID,
		)
		return "", perrors.WithCode(code.ErrProofBuildFailed, "wechat app is disabled: %s", payload.AppID)
	}
	if wechatApp.Cred == nil || wechatApp.Cred.Auth == nil {
		l.Errorw("微信应用凭据缺失",
			"action", logger.ActionLogin,
			"credential_kind", string(method.CredentialKindWechatMinip),
			"app_id", payload.AppID,
		)
		return "", perrors.WithCode(code.ErrProofBuildFailed, "wechat app credentials not found")
	}

	appSecretPlain, err := b.secretVault.Decrypt(ctx, wechatApp.Cred.Auth.AppSecretCipher)
	if err != nil {
		l.Errorw("解密应用密钥失败",
			"action", logger.ActionLogin,
			"credential_kind", string(method.CredentialKindWechatMinip),
			"app_id", payload.AppID,
			"error", err.Error(),
		)
		return "", perrors.WithCode(code.ErrProofBuildFailed, "failed to decrypt app secret: %v", err)
	}
	return string(appSecretPlain), nil
}

// wecomBuilder 企业微信登录方式构造器
type wecomBuilder struct {
	wechatAppQuerier idpPort.Repository
	secretVault      idpPort.SecretVault
	config           WecomConfig
}

// NewWecomBuilder 创建企业微信登录方式构造器
func newWecomBuilder(repo idpPort.Repository, vault idpPort.SecretVault, config WecomConfig) Builder {
	return &wecomBuilder{wechatAppQuerier: repo, secretVault: vault, config: config}
}

// CredentialKind 返回认证证明类型
func (*wecomBuilder) CredentialKind() method.CredentialKind {
	return method.CredentialKindWecom
}

// Build 构建企业微信登录方式
func (b *wecomBuilder) Build(ctx context.Context, payload method.Payload, common method.CommonPayload) (authentication.AuthCredential, error) {
	wecomPayload, ok := payload.(method.WecomPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrProofBuildFailed, "invalid wecom payload")
	}
	// 准备企业微信应用配置
	appConfig, err := b.prepareWecomAppConfig(ctx, wecomPayload)
	if err != nil {
		return nil, err
	}

	// 构建企业微信认证凭据
	return authentication.NewWecomCredential(authentication.WecomProofSpec{
		TenantID:   common.TenantID,
		RemoteIP:   common.RemoteIP,
		UserAgent:  common.UserAgent,
		CorpID:     wecomPayload.CorpID,
		AgentID:    appConfig.agentID,
		CorpSecret: appConfig.corpSecret,
		Code:       wecomPayload.Code,
	})
}

// wecomAppConfig 企业微信应用配置
type wecomAppConfig struct {
	agentID    string
	corpSecret string
}

// prepareWecomAppConfig 准备企业微信应用配置
func (b *wecomBuilder) prepareWecomAppConfig(ctx context.Context, payload method.WecomPayload) (wecomAppConfig, error) {
	l := logger.L(ctx)
	if b.wechatAppQuerier == nil || b.secretVault == nil {
		l.Errorw("企业微信应用配置服务不可用",
			"action", logger.ActionLogin,
			"credential_kind", string(method.CredentialKindWecom),
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrProofBuildFailed, "wecom app configuration service not available")
	}

	agentID := strings.TrimSpace(b.config.AgentID)
	if agentID == "" {
		l.Errorw("企业微信应用 agent_id 未配置",
			"action", logger.ActionLogin,
			"credential_kind", string(method.CredentialKindWecom),
			"corp_id", payload.CorpID,
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrProofBuildFailed, "wecom agent_id is required in server configuration")
	}

	wechatApp, err := b.wechatAppQuerier.GetByAppID(ctx, payload.CorpID)
	if err != nil {
		l.Errorw("查询企业微信应用配置失败",
			"action", logger.ActionLogin,
			"credential_kind", string(method.CredentialKindWecom),
			"corp_id", payload.CorpID,
			"error", err.Error(),
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrProofBuildFailed, "failed to query wecom app: %v", err)
	}
	if wechatApp == nil {
		l.Warnw("企业微信应用不存在",
			"action", logger.ActionLogin,
			"credential_kind", string(method.CredentialKindWecom),
			"corp_id", payload.CorpID,
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrProofBuildFailed, "wecom app not found: %s", payload.CorpID)
	}
	if !wechatApp.IsEnabled() {
		l.Warnw("企业微信应用已禁用",
			"action", logger.ActionLogin,
			"credential_kind", string(method.CredentialKindWecom),
			"corp_id", payload.CorpID,
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrProofBuildFailed, "wecom app is disabled: %s", payload.CorpID)
	}
	if wechatApp.Cred == nil || wechatApp.Cred.Auth == nil {
		l.Errorw("企业微信应用凭据缺失",
			"action", logger.ActionLogin,
			"credential_kind", string(method.CredentialKindWecom),
			"corp_id", payload.CorpID,
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrProofBuildFailed, "wecom app credentials not found")
	}

	corpSecretPlain, err := b.secretVault.Decrypt(ctx, wechatApp.Cred.Auth.AppSecretCipher)
	if err != nil {
		l.Errorw("解密企业微信应用密钥失败",
			"action", logger.ActionLogin,
			"credential_kind", string(method.CredentialKindWecom),
			"corp_id", payload.CorpID,
			"error", err.Error(),
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrProofBuildFailed, "failed to decrypt wecom corp secret: %v", err)
	}
	return wecomAppConfig{agentID: agentID, corpSecret: string(corpSecretPlain)}, nil
}
