package linking

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authnexternal "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/externalidentity"
	idpresolver "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
	idpidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// PrepareLink 准备微信小程序登录身份。
func (in LinkWechatMiniInput) prepareLink(ctx context.Context, deps linkPrepareDeps, userID meta.ID) (preparedLink, error) {
	// 检查微信小程序应用 ID 和认证码是否有效。
	appID := strings.TrimSpace(in.AppID)
	jsCode := strings.TrimSpace(in.Code)
	if appID == "" || jsCode == "" {
		return preparedLink{}, perrors.WithCode(code.ErrInvalidArgument, "app_id and code are required")
	}
	// 检查微信小程序身份提供者是否配置。
	if deps.resolver == nil {
		return preparedLink{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app configuration service not available")
	}
	identity, err := deps.resolver.Resolve(ctx, idpresolver.ResolveRequest{
		Provider: idpidentity.ProviderWechatMinip,
		Realm:    appID,
		Code:     jsCode,
	})
	if err != nil {
		return preparedLink{}, authnexternal.MapLinkingError(err)
	}
	key, err := authnexternal.ProviderKey(identity)
	if err != nil {
		return preparedLink{}, perrors.WithCode(code.ErrInvalidCredential, "failed to map wechat external identity: %v", err)
	}

	// 构建已验证登录身份。
	return preparedLink{
		key:                     key,
		build:                   verifiedIdentityBuild(userID, key, identity.VerifiedAt()),
		requireGlobalUniqueness: true,
	}, nil
}
