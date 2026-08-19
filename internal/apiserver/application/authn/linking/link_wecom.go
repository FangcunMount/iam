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

// PrepareLink 准备企业微信登录身份。
func (in LinkWecomInput) prepareLink(ctx context.Context, deps linkPrepareDeps, userID meta.ID) (preparedLink, error) {
	// 检查企业微信 Corp ID 和认证码是否有效；Agent ID 由 IDP 自己管理。
	corpID := strings.TrimSpace(in.CorpID)
	authCode := strings.TrimSpace(in.Code)
	if corpID == "" || authCode == "" {
		return preparedLink{}, perrors.WithCode(code.ErrInvalidArgument, "corp_id, code and wecom agent_id are required")
	}
	// 检查企业微信身份提供者是否配置。
	if deps.resolver == nil {
		return preparedLink{}, perrors.WithCode(code.ErrInvalidArgument, "wecom app configuration service not available")
	}
	identity, err := deps.resolver.Resolve(ctx, idpresolver.ResolveRequest{
		Provider: idpidentity.ProviderWecom,
		Realm:    corpID,
		Code:     authCode,
	})
	if err != nil {
		return preparedLink{}, authnexternal.MapLinkingError(err)
	}
	key, err := authnexternal.ProviderKey(identity)
	if err != nil {
		return preparedLink{}, perrors.WithCode(code.ErrInvalidCredential, "failed to map wecom external identity: %v", err)
	}

	// 构建已验证登录身份。
	return preparedLink{
		key:   key,
		build: verifiedIdentityBuild(userID, key, identity.VerifiedAt()),
	}, nil
}
