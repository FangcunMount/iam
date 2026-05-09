package token

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
)

// ====================================================
// ================== Driving Ports ===================
// ====================================================
// issuerPort 聚合 token 签发/撤销能力，仅供 token 包内部装配。
type issuerPort interface {
	sessionTokenIssuerPort
	serviceTokenIssuerPort
	accessRevokerPort
}

// ====================================================
// ================== Implementation ==================
// ====================================================
// issuer 是 token 包内部的签发门面，实际职责委托给更小的用例组件。
type issuer struct {
	sessionIssuer *sessionTokenIssuer
	serviceIssuer serviceTokenIssuerPort
	accessRevoker accessRevokerPort
}

// 确保 issuer 实现 issuerPort 接口。
var _ issuerPort = (*issuer)(nil)

// newIssuer 创建 token 签发器。
func newIssuer(
	tokenCodec AccessTokenCodec,
	tokenStore Store,
	sessionManager SessionManager,
	claimMapper ClaimMapper,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) *issuer {
	claimMapper = normalizeClaimMapper(claimMapper)
	pairIssuer := newSessionTokenPairIssuer(tokenCodec, tokenStore, claimMapper, accessTTL, refreshTTL)
	return &issuer{
		sessionIssuer: newSessionTokenIssuer(sessionManager, pairIssuer, refreshTTL),
		serviceIssuer: newServiceTokenIssuer(tokenCodec, accessTTL),
		accessRevoker: newAccessTokenRevoker(tokenCodec, tokenStore, sessionManager),
	}
}

// sessionTokenPairIssuer 返回基于既有 session 签发 token pair 的内部协作者。
func (s *issuer) sessionTokenPairIssuer() sessionTokenPairIssuerPort {
	if s == nil || s.sessionIssuer == nil {
		return nil
	}
	return s.sessionIssuer.pairIssuer
}

// IssueToken 颁发用户会话令牌。
func (s *issuer) IssueToken(ctx context.Context, principal *authentication.Principal) (*TokenPair, error) {
	return s.sessionIssuer.IssueToken(ctx, principal)
}

// IssueServiceToken 颁发服务令牌。
func (s *issuer) IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*TokenPair, error) {
	return s.serviceIssuer.IssueServiceToken(ctx, subject, audience, attributes, ttl)
}

// RevokeAccessToken 撤销访问令牌。
func (s *issuer) RevokeAccessToken(ctx context.Context, tokenValue string) error {
	return s.accessRevoker.RevokeAccessToken(ctx, tokenValue)
}
