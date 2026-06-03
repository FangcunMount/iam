package proof

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/challenge"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/method"
	idpprepare "github.com/FangcunMount/iam/v2/internal/apiserver/application/idp/prepare"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	idpPort "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

type wechatScanBuilder struct {
	deps        idpprepare.Dependencies
	oauthStates challenge.WechatOpenOAuthStateVerifier
}

func newWechatScanBuilder(repo idpPort.Repository, vault idpPort.SecretVault, oauthStates challenge.WechatOpenOAuthStateVerifier) Builder {
	return &wechatScanBuilder{
		deps:        idpprepare.Dependencies{Apps: repo, Vault: vault},
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

	appSecret, err := idpprepare.ResolveAppSecret(ctx, b.deps, idpprepare.Options{
		Provider:        idpprepare.ProviderWechat,
		Surface:         idpprepare.SurfaceLoginProof,
		AppID:           scanPayload.AppID,
		CredentialKind:  string(authentication.CredentialKindWechatOpen),
		ExpectedAppType: idpPort.OpenPlatformWebsite,
	})
	if err != nil {
		return nil, err
	}

	return authentication.NewWechatOpenCredential(authentication.WechatOpenProofSpec{
		TenantID:  common.TenantID,
		RemoteIP:  common.RemoteIP,
		UserAgent: common.UserAgent,
		AppID:     scanPayload.AppID,
		AppSecret: appSecret,
		Code:      scanPayload.Code,
	})
}
