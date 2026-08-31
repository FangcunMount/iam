package proof

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authnexternal "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin/method"
	idpresolver "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	idpidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// wechatBuilder 微信小程序登录方式构造器
type wechatBuilder struct {
	resolver idpresolver.Resolver
}

// newWechatBuilder 创建微信小程序登录方式构造器
func newWechatBuilder(resolver idpresolver.Resolver) Builder {
	return &wechatBuilder{resolver: resolver}
}

// CredentialKind 返回认证证明类型
func (*wechatBuilder) CredentialKind() method.CredentialKind {
	return method.CredentialKindWechatMinip
}

// Build 构建微信小程序登录方式
func (b *wechatBuilder) Build(ctx context.Context, payload method.Payload, common method.CommonPayload) (authentication.AuthCredential, error) {
	// 验证微信小程序登录方式凭证是否有效
	wechatMiniPayload, ok := payload.(method.WechatMiniPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrProofBuildFailed, "invalid wechat payload")
	}

	// 检查微信小程序登录方式凭证解析器是否可用
	if b.resolver == nil {
		return nil, perrors.WithCode(code.ErrProofBuildFailed, "wechat app configuration service not available")
	}

	// 解析微信小程序登录方式凭证，返回微信身份信息
	resolved, err := b.resolver.Resolve(ctx, idpresolver.ResolveRequest{
		Provider: idpidentity.ProviderWechatMinip,
		Realm:    wechatMiniPayload.AppID,
		Code:     wechatMiniPayload.JSCode,
	})
	if err != nil {
		return nil, authnexternal.MapLoginProofError(ctx, err, string(method.CredentialKindWechatMinip))
	}

	// 映射微信小程序登录方式凭证，返回微信身份信息
	wechatIdentity, err := authnexternal.Wechat(resolved)
	if err != nil {
		return nil, perrors.WithCode(code.ErrProofBuildFailed, "failed to map wechat external identity: %v", err)
	}

	return authentication.NewWechatMiniCredential(authentication.WechatMiniProofSpec{
		TenantID:  common.TenantID,
		RemoteIP:  common.RemoteIP,
		UserAgent: common.UserAgent,
		AppID:     wechatIdentity.Realm,
		OpenID:    wechatIdentity.OpenID,
		UnionID:   wechatIdentity.UnionID,
	})
}

// wecomBuilder 企业微信登录方式构造器
type wecomBuilder struct {
	resolver idpresolver.Resolver
}

// newWecomBuilder 创建企业微信登录方式构造器
func newWecomBuilder(resolver idpresolver.Resolver) Builder {
	return &wecomBuilder{resolver: resolver}
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

	if b.resolver == nil {
		return nil, perrors.WithCode(code.ErrProofBuildFailed, "wecom app configuration service not available")
	}
	resolved, err := b.resolver.Resolve(ctx, idpresolver.ResolveRequest{
		Provider: idpidentity.ProviderWecom,
		Realm:    wecomPayload.CorpID,
		Code:     wecomPayload.Code,
	})
	if err != nil {
		return nil, authnexternal.MapLoginProofError(ctx, err, string(method.CredentialKindWecom))
	}
	identity, err := authnexternal.Wecom(resolved)
	if err != nil {
		return nil, perrors.WithCode(code.ErrProofBuildFailed, "failed to map wecom external identity: %v", err)
	}

	return authentication.NewWecomCredential(authentication.WecomProofSpec{
		TenantID:   common.TenantID,
		RemoteIP:   common.RemoteIP,
		UserAgent:  common.UserAgent,
		CorpID:     identity.Realm,
		UserID:     identity.UserID,
		OpenUserID: identity.OpenUserID,
	})
}
