package onboarding

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	accountDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	idpPort "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

type wechatIdentityResolver struct {
	idp              authentication.IdentityProvider
	wechatAppQuerier idpPort.Repository
	secretVault      idpPort.SecretVault
}

type wechatIdentity struct {
	OpenID  string
	UnionID string
}

func newWechatIdentityResolver(
	idp authentication.IdentityProvider,
	wechatAppQuerier idpPort.Repository,
	secretVault idpPort.SecretVault,
) *wechatIdentityResolver {
	return &wechatIdentityResolver{
		idp:              idp,
		wechatAppQuerier: wechatAppQuerier,
		secretVault:      secretVault,
	}
}

func (r *wechatIdentityResolver) ResolveMiniProgram(ctx context.Context, req OnboardingRequest) (wechatIdentity, error) {
	if req.AccountType != accountDomain.TypeWcMinip {
		return wechatIdentity{}, nil
	}
	if req.WechatOpenID != nil && *req.WechatOpenID != "" {
		identity := wechatIdentity{OpenID: *req.WechatOpenID}
		if req.WechatUnionID != nil {
			identity.UnionID = *req.WechatUnionID
		}
		return identity, nil
	}
	if req.WechatAppID == nil || *req.WechatAppID == "" || req.WechatJsCode == nil || *req.WechatJsCode == "" {
		return wechatIdentity{}, nil
	}
	if r == nil || r.wechatAppQuerier == nil || r.secretVault == nil {
		return wechatIdentity{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app configuration service not available")
	}

	appSecret, err := r.resolveAppSecret(ctx, *req.WechatAppID)
	if err != nil {
		return wechatIdentity{}, err
	}

	openID, unionID, err := r.idp.ExchangeWxMinipCode(ctx, *req.WechatAppID, appSecret, *req.WechatJsCode)
	if err != nil {
		return wechatIdentity{}, perrors.WithCode(code.ErrInvalidCredential, "failed to call wechat code2session: %v", err)
	}
	return wechatIdentity{OpenID: openID, UnionID: unionID}, nil
}

func (r *wechatIdentityResolver) resolveAppSecret(ctx context.Context, appID string) (string, error) {
	wechatApp, err := r.wechatAppQuerier.GetByAppID(ctx, appID)
	if err != nil {
		return "", perrors.WithCode(code.ErrInvalidArgument, "failed to query wechat app: %v", err)
	}
	if wechatApp == nil {
		return "", perrors.WithCode(code.ErrInvalidArgument, "wechat app not found: %s", appID)
	}
	if !wechatApp.IsEnabled() {
		return "", perrors.WithCode(code.ErrInvalidArgument, "wechat app is disabled: %s", appID)
	}
	if wechatApp.Cred == nil || wechatApp.Cred.Auth == nil {
		return "", perrors.WithCode(code.ErrInvalidArgument, "wechat app credentials not found")
	}

	appSecretPlain, err := r.secretVault.Decrypt(ctx, wechatApp.Cred.Auth.AppSecretCipher)
	if err != nil {
		return "", perrors.WithCode(code.ErrInvalidArgument, "failed to decrypt app secret: %v", err)
	}
	return string(appSecretPlain), nil
}

func prepareWechatIdentity(req OnboardingRequest, identity wechatIdentity) OnboardingRequest {
	prepared := req
	if identity.OpenID == "" {
		return prepared
	}
	if prepared.WechatOpenID == nil {
		prepared.WechatOpenID = stringPtr(identity.OpenID)
	}
	if identity.UnionID != "" && prepared.WechatUnionID == nil {
		prepared.WechatUnionID = stringPtr(identity.UnionID)
	}
	prepared.WechatJsCode = nil
	return prepared
}

func stringPtr(v string) *string {
	return &v
}
