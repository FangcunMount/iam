package linking

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// PrepareLink 准备企业微信登录身份。
func (in LinkWecomInput) prepareLink(ctx context.Context, deps linkPrepareDeps, userID meta.ID) (preparedLink, error) {
	// 检查企业微信 Corp ID、认证码和企业微信 Agent ID 是否有效。
	corpID := strings.TrimSpace(in.CorpID)
	authCode := strings.TrimSpace(in.Code)
	agentID := strings.TrimSpace(deps.wecomAgentID)
	if corpID == "" || authCode == "" || agentID == "" {
		return preparedLink{}, perrors.WithCode(code.ErrInvalidArgument, "corp_id, code and wecom agent_id are required")
	}
	// 检查企业微信身份提供者是否配置。
	if deps.idp == nil {
		return preparedLink{}, perrors.WithCode(code.ErrInvalidArgument, "wecom app configuration service not available")
	}
	// 解析企业微信应用密钥。
	corpSecret, err := deps.resolveAppSecret(ctx, corpID, "wecom")
	if err != nil {
		return preparedLink{}, err
	}
	// 交换企业微信认证码。
	_, userIDInWecom, err := deps.idp.ExchangeWecomCode(ctx, corpID, agentID, corpSecret, authCode)
	if err != nil {
		return preparedLink{}, perrors.WithCode(code.ErrInvalidCredential, "failed to exchange wecom code: %v", err)
	}
	// 构建提供者密钥。
	key := loginidentity.WecomProviderKey(corpID, userIDInWecom)

	// 构建已验证登录身份。
	verifiedAt := deps.currentTime()
	return preparedLink{
		key:   key,
		build: verifiedIdentityBuild(userID, key, verifiedAt),
	}, nil
}
