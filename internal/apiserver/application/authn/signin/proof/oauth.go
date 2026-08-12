package proof

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin/method"
	idpprepare "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/prepare"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	idpPort "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// wechatBuilder 微信小程序登录方式构造器
type wechatBuilder struct {
	deps idpprepare.Dependencies
}

// newWechatBuilder 创建微信小程序登录方式构造器
func newWechatBuilder(repo idpPort.Repository, vault idpPort.SecretVault) Builder {
	return &wechatBuilder{deps: idpprepare.Dependencies{Apps: repo, Vault: vault}}
}

// CredentialKind 返回认证证明类型
func (*wechatBuilder) CredentialKind() method.CredentialKind {
	return method.CredentialKindWechatMinip
}

// Build 构建微信小程序登录方式
func (b *wechatBuilder) Build(ctx context.Context, payload method.Payload, common method.CommonPayload) (authentication.AuthCredential, error) {
	wechatMiniPayload, ok := payload.(method.WechatMiniPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrProofBuildFailed, "invalid wechat payload")
	}

	appSecret, err := idpprepare.ResolveAppSecret(ctx, b.deps, idpprepare.Options{
		Provider:       idpprepare.ProviderWechat,
		Surface:        idpprepare.SurfaceLoginProof,
		AppID:          wechatMiniPayload.AppID,
		CredentialKind: string(method.CredentialKindWechatMinip),
	})
	if err != nil {
		return nil, err
	}

	return authentication.NewWechatMiniCredential(authentication.WechatMiniProofSpec{
		TenantID:  common.TenantID,
		RemoteIP:  common.RemoteIP,
		UserAgent: common.UserAgent,
		AppID:     wechatMiniPayload.AppID,
		AppSecret: appSecret,
		Code:      wechatMiniPayload.JSCode,
	})
}

// wecomBuilder 企业微信登录方式构造器
type wecomBuilder struct {
	deps   idpprepare.Dependencies
	config WecomConfig
}

// newWecomBuilder 创建企业微信登录方式构造器
func newWecomBuilder(repo idpPort.Repository, vault idpPort.SecretVault, config WecomConfig) Builder {
	return &wecomBuilder{
		deps:   idpprepare.Dependencies{Apps: repo, Vault: vault},
		config: config,
	}
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

	appConfig, err := b.prepareWecomAppConfig(ctx, wecomPayload)
	if err != nil {
		return nil, err
	}

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

type wecomAppConfig struct {
	agentID    string
	corpSecret string
}

func (b *wecomBuilder) prepareWecomAppConfig(ctx context.Context, payload method.WecomPayload) (wecomAppConfig, error) {
	agentID := strings.TrimSpace(b.config.AgentID)
	if agentID == "" {
		logger.L(ctx).Errorw("企业微信应用 agent_id 未配置",
			"action", logger.ActionLogin,
			"credential_kind", string(method.CredentialKindWecom),
			"corp_id", payload.CorpID,
		)
		return wecomAppConfig{}, perrors.WithCode(code.ErrProofBuildFailed, "wecom agent_id is required in server configuration")
	}

	corpSecret, err := idpprepare.ResolveAppSecret(ctx, b.deps, idpprepare.Options{
		Provider:       idpprepare.ProviderWecom,
		Surface:        idpprepare.SurfaceLoginProof,
		AppID:          payload.CorpID,
		CredentialKind: string(method.CredentialKindWecom),
	})
	if err != nil {
		return wecomAppConfig{}, err
	}
	return wecomAppConfig{agentID: agentID, corpSecret: corpSecret}, nil
}
