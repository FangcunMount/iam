package proof

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/challenge"
	authnexternal "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin/method"
	idpresolver "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	idpidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

type wechatScanBuilder struct {
	resolver    idpresolver.Resolver
	oauthStates challenge.WechatOpenOAuthStateVerifier
}

func newWechatScanBuilder(resolver idpresolver.Resolver, oauthStates challenge.WechatOpenOAuthStateVerifier) Builder {
	return &wechatScanBuilder{
		resolver:    resolver,
		oauthStates: oauthStates,
	}
}

func (*wechatScanBuilder) CredentialKind() method.CredentialKind {
	return method.CredentialKindWechatScan
}

func (b *wechatScanBuilder) Build(ctx context.Context, payload method.Payload, common method.CommonPayload) (authentication.AuthCredential, error) {
	scanPayload, ok := payload.(method.WechatScanPayload)
	if !ok {
		return nil, perrors.WithCode(code.ErrProofBuildFailed, "invalid wechat scan payload")
	}
	if b.oauthStates == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "oauth state verifier is not configured")
	}

	oauthCtx, err := b.oauthStates.VerifyAndConsumeWechatOpenLogin(ctx, scanPayload.State)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(oauthCtx.AppID) != strings.TrimSpace(scanPayload.AppID) {
		return nil, perrors.WithCode(code.ErrStateMismatch, "oauth state app_id mismatch")
	}

	if b.resolver == nil {
		return nil, perrors.WithCode(code.ErrProofBuildFailed, "wechat app configuration service not available")
	}
	resolved, err := b.resolver.Resolve(ctx, idpresolver.ResolveRequest{
		Provider: idpidentity.ProviderWechatOpen,
		Realm:    scanPayload.AppID,
		Code:     scanPayload.Code,
	})
	if err != nil {
		return nil, authnexternal.MapLoginProofError(ctx, err, string(authentication.CredentialKindWechatOpen))
	}
	identity, err := authnexternal.Wechat(resolved)
	if err != nil {
		return nil, perrors.WithCode(code.ErrProofBuildFailed, "failed to map wechat external identity: %v", err)
	}

	return authentication.NewWechatOpenCredential(authentication.WechatOpenProofSpec{
		TenantID:  common.TenantID,
		RemoteIP:  common.RemoteIP,
		UserAgent: common.UserAgent,
		AppID:     identity.Realm,
		OpenID:    identity.OpenID,
		UnionID:   identity.UnionID,
	})
}
