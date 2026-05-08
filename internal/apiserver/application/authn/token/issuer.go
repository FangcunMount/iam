package token

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// issuer 实现 Issuer 接口
type issuer struct {
	tokenCodec     AccessTokenCodec
	tokenStore     Store
	sessionManager SessionManager
	pairIssuer     SessionTokenPairIssuer
	accessTTL      time.Duration
	refreshTTL     time.Duration
}

// 确保 issuer 实现 Issuer 接口
var _ Issuer = (*issuer)(nil)

// NewIssuer 创建 token 签发器。
func NewIssuer(
	tokenCodec AccessTokenCodec,
	tokenStore Store,
	sessionManager SessionManager,
	claimMapper ClaimMapper,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) *issuer {
	claimMapper = normalizeClaimMapper(claimMapper)
	return &issuer{
		tokenCodec:     tokenCodec,
		tokenStore:     tokenStore,
		sessionManager: sessionManager,
		pairIssuer:     newSessionTokenPairIssuer(tokenCodec, tokenStore, claimMapper, accessTTL, refreshTTL),
		accessTTL:      accessTTL,
		refreshTTL:     refreshTTL,
	}
}

// IssueToken 颁发令牌
func (s *issuer) IssueToken(ctx context.Context, principal *authentication.Principal) (*TokenPair, error) {
	if principal == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "principal is required")
	}

	sessionExpiresAt := time.Now().Add(s.refreshTTL)
	sess, err := s.sessionManager.Create(ctx, principal, sessionExpiresAt)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to create session")
	}

	return s.pairIssuer.IssueTokenPair(ctx, principal, sess)
}

// IssueTokenPair 颁发令牌对
func (s *issuer) IssueTokenPair(ctx context.Context, principal *authentication.Principal, sess *sessiondomain.Session) (*TokenPair, error) {
	if s == nil || s.pairIssuer == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "session token pair issuer is not configured")
	}
	return s.pairIssuer.IssueTokenPair(ctx, principal, sess)
}

// IssueServiceToken 颁发服务令牌
func (s *issuer) IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*TokenPair, error) {
	if subject == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "subject is required")
	}
	if ttl <= 0 {
		ttl = s.accessTTL
	}

	serviceToken, err := s.tokenCodec.IssueServiceToken(ctx, subject, audience, attributes, ttl)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate service token")
	}
	return NewTokenPair(serviceToken, nil), nil
}

// RevokeAccessToken 撤销访问令牌
func (s *issuer) RevokeAccessToken(ctx context.Context, tokenValue string) error {
	claims, err := s.tokenCodec.VerifyAccessToken(ctx, tokenValue)
	if err != nil {
		return perrors.WrapC(err, code.ErrTokenInvalid, "failed to parse token for revocation")
	}
	if claims.IsExpired() {
		return nil
	}

	expiry := time.Until(claims.ExpiresAt)
	if expiry <= 0 {
		return nil
	}
	if err := s.tokenStore.MarkAccessTokenRevoked(ctx, claims.TokenID, expiry); err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to mark access token revoked")
	}
	if claims.SessionID != "" {
		if err := s.sessionManager.Revoke(ctx, claims.SessionID, "access_token_revoked", claims.Subject); err != nil {
			return perrors.WrapC(err, code.ErrInternalServerError, "failed to revoke token session")
		}
	}
	return nil
}

// cloneAnyMap 克隆任意映射
func cloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
