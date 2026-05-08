package token

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/security/sanitize"
)

// refresher 刷新令牌
type refresher struct {
	pairIssuer     SessionTokenPairIssuer
	tokenStore     Store
	sessionManager SessionManager
	accessChecker  SubjectAccessEvaluator
	claimMapper    ClaimMapper
}

// 确保 refresher 实现 Refresher 接口
var _ Refresher = (*refresher)(nil)

// NewRefresher 创建 refresher
func NewRefresher(
	pairIssuer SessionTokenPairIssuer,
	tokenStore Store,
	sessionManager SessionManager,
	accessChecker SubjectAccessEvaluator,
	claimMapper ClaimMapper,
) Refresher {
	return &refresher{
		pairIssuer:     pairIssuer,
		tokenStore:     tokenStore,
		sessionManager: sessionManager,
		accessChecker:  accessChecker,
		claimMapper:    normalizeClaimMapper(claimMapper),
	}
}

// RefreshToken 刷新访问令牌
// 职责：根据 refresh token 刷新 access token 和 refresh token。
// 返回值必须包含 access token 和 refresh token。
func (s *refresher) RefreshToken(ctx context.Context, refreshTokenValue string) (*TokenPair, error) {
	// 加载刷新令牌
	refreshToken, err := s.loadRefreshToken(ctx, refreshTokenValue)
	if err != nil {
		return nil, err
	}

	// 加载活跃会话
	sess, err := s.loadActiveSession(ctx, refreshToken.SessionID)
	if err != nil {
		return nil, err
	}

	// 确保主体访问权限允许
	if err := s.ensureSubjectAccessAllowed(ctx, refreshToken); err != nil {
		return nil, err
	}

	// 确保刷新令牌可用
	if err := s.ensureRefreshTokenUsable(ctx, refreshTokenValue, refreshToken); err != nil {
		return nil, err
	}

	// 从刷新令牌创建主体
	principal := s.principalFromRefreshToken(refreshToken)
	newTokenPair, err := s.issueRotatedTokenPair(ctx, principal, sess)
	if err != nil {
		return nil, err
	}

	// 删除过期刷新令牌
	s.deleteStaleRefreshToken(ctx, refreshTokenValue)
	if err := s.extendSessionToRefreshExpiry(ctx, sess.SessionID, newTokenPair.RefreshToken.ExpiresAt); err != nil {
		return nil, err
	}

	// 返回新的令牌对
	return newTokenPair, nil
}

// RevokeRefreshToken 删除 refresh token；如果能解析到 session，则同步撤销该 session。
// 职责：撤销单个 refresh token。
func (s *refresher) RevokeRefreshToken(ctx context.Context, refreshTokenValue string) error {
	// 加载刷新令牌
	refreshToken, err := s.tokenStore.GetRefreshToken(ctx, refreshTokenValue)
	if err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to load refresh token")
	}
	// 如果刷新令牌存在且有会话ID，则撤销会话
	if refreshToken != nil && refreshToken.SessionID != "" {
		if err := s.sessionManager.Revoke(ctx, refreshToken.SessionID, "refresh_token_revoked", refreshToken.UserID.String()); err != nil {
			return perrors.WrapC(err, code.ErrInternalServerError, "failed to revoke refresh token session")
		}
	}
	// 删除刷新令牌
	if err := s.tokenStore.DeleteRefreshToken(ctx, refreshTokenValue); err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to revoke refresh token")
	}
	return nil
}

// loadRefreshToken 加载刷新令牌
func (s *refresher) loadRefreshToken(ctx context.Context, refreshTokenValue string) (*Token, error) {
	refreshToken, err := s.tokenStore.GetRefreshToken(ctx, refreshTokenValue)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrTokenInvalid, "refresh token not found or invalid")
	}
	if refreshToken == nil {
		return nil, perrors.WithCode(code.ErrTokenInvalid, "refresh token not found")
	}
	return refreshToken, nil
}

// loadActiveSession 加载活跃会话
func (s *refresher) loadActiveSession(ctx context.Context, sessionID string) (*sessiondomain.Session, error) {
	sess, err := s.sessionManager.Get(ctx, sessionID)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to load session")
	}
	if sess == nil || !sess.IsActive() {
		return nil, perrors.WithCode(code.ErrTokenInvalid, "session has been revoked or expired")
	}
	return sess, nil
}

// ensureSubjectAccessAllowed 确保主体访问权限允许
func (s *refresher) ensureSubjectAccessAllowed(ctx context.Context, refreshToken *Token) error {
	decision, err := s.accessChecker.Evaluate(ctx, refreshToken.UserID, refreshToken.AccountID)
	if err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to evaluate subject access")
	}
	if !decision.IsAllowed() {
		return subjectAccessError(decision.Status)
	}
	return nil
}

// ensureRefreshTokenUsable 确保刷新令牌可用
func (s *refresher) ensureRefreshTokenUsable(ctx context.Context, refreshTokenValue string, refreshToken *Token) error {
	if !refreshToken.IsExpired() {
		return nil
	}
	_ = s.tokenStore.DeleteRefreshToken(ctx, refreshTokenValue)
	return perrors.WithCode(code.ErrExpired, "refresh token has expired")
}

// principalFromRefreshToken 从刷新令牌创建主体
func (s *refresher) principalFromRefreshToken(refreshToken *Token) *authentication.Principal {
	amr := refreshToken.AMR
	if len(amr) == 0 {
		amr = []string{"jwt"}
	}
	claims := s.claimMapper.Decode(refreshToken.SessionClaims)
	if claims == nil {
		claims = make(map[string]any)
	}
	return &authentication.Principal{
		UserID:    refreshToken.UserID,
		AccountID: refreshToken.AccountID,
		TenantID:  refreshToken.TenantID,
		SessionID: refreshToken.SessionID,
		AMR:       amr,
		Claims:    claims,
	}
}

// issueRotatedTokenPair 颁发新的令牌对
func (s *refresher) issueRotatedTokenPair(ctx context.Context, principal *authentication.Principal, sess *sessiondomain.Session) (*TokenPair, error) {
	if s.pairIssuer == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "session token pair issuer is not configured")
	}
	newTokenPair, err := s.pairIssuer.IssueTokenPair(ctx, principal, sess)
	if err != nil {
		return nil, err
	}
	if newTokenPair == nil || newTokenPair.AccessToken == nil || newTokenPair.RefreshToken == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "session token pair issuer returned incomplete token pair")
	}
	return newTokenPair, nil
}

// deleteStaleRefreshToken 删除过期刷新令牌
func (s *refresher) deleteStaleRefreshToken(ctx context.Context, refreshTokenValue string) {
	if err := s.tokenStore.DeleteRefreshToken(ctx, refreshTokenValue); err != nil {
		logger.L(ctx).Errorw("failed to delete stale refresh token after rotation",
			"action", logger.ActionRefresh,
			"resource", logger.ResourceToken,
			"token_type", "refresh",
			"token_hint", sanitize.MaskToken(refreshTokenValue),
			"error", err.Error(),
		)
	}
}

// extendSessionToRefreshExpiry 延长会话到刷新令牌过期时间
func (s *refresher) extendSessionToRefreshExpiry(ctx context.Context, sessionID string, expiresAt time.Time) error {
	if err := s.sessionManager.Extend(ctx, sessionID, expiresAt); err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to extend session ttl")
	}
	return nil
}

// subjectAccessError 转换 subject access 状态为错误
func subjectAccessError(status sessiondomain.SubjectAccessStatus) error {
	switch status {
	case sessiondomain.SubjectAccessBlocked:
		return perrors.WithCode(code.ErrUserBlocked, "user is blocked")
	case sessiondomain.SubjectAccessDisabled:
		return perrors.WithCode(code.ErrCredentialDisabled, "account is disabled")
	case sessiondomain.SubjectAccessLocked:
		return perrors.WithCode(code.ErrCredentialLocked, "account is locked")
	default:
		return perrors.WithCode(code.ErrUserInactive, "subject is inactive")
	}
}
