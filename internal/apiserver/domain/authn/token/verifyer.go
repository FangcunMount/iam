package token

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	sessiondomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/security/sanitize"
)

// TokenVerifyer 令牌验证者
type TokenVerifyer struct {
	tokenGenerator TokenGenerator // JWT 生成器
	tokenStore     TokenStore     // 令牌存储（Redis）
	sessionManager SessionManager
	accessChecker  SubjectAccessEvaluator
}

// NewTokenVerifyer 创建令牌验证者
func NewTokenVerifyer(
	tokenGenerator TokenGenerator,
	tokenStore TokenStore,
	sessionManager SessionManager,
	accessChecker SubjectAccessEvaluator,
) *TokenVerifyer {
	return &TokenVerifyer{
		tokenGenerator: tokenGenerator,
		tokenStore:     tokenStore,
		sessionManager: sessionManager,
		accessChecker:  accessChecker,
	}
}

// VerifyAccessToken 验证访问令牌
func (s *TokenVerifyer) VerifyAccessToken(ctx context.Context, tokenValue string) (*TokenClaims, error) {
	// 解析 JWT
	claims, err := s.tokenGenerator.ParseAccessToken(ctx, tokenValue)
	if err != nil {
		log.Warnw("failed to parse access token",
			"error", err,
			"token_hint", sanitize.MaskToken(tokenValue),
		)
		return nil, perrors.WrapC(err, code.ErrTokenInvalid, "failed to parse access token")
	}

	// 检查过期
	if claims.IsExpired() {
		log.Infow("access token has expired",
			"token_id", claims.TokenID,
			"expires_at", claims.ExpiresAt,
			"user_id", claims.UserID,
		)
		return nil, perrors.WithCode(code.ErrExpired, "access token has expired")
	}

	if claims.TokenType == TokenTypeService {
		return claims, nil
	}

	// 检查访问令牌是否已撤销
	isRevoked, err := s.tokenStore.IsAccessTokenRevoked(ctx, claims.TokenID)
	if err != nil {
		log.Errorw("failed to check revoked access token",
			"error", err,
			"token_id", claims.TokenID,
		)
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to check revoked access token")
	}
	if isRevoked {
		log.Warnw("access token has been revoked",
			"token_id", claims.TokenID,
			"user_id", claims.UserID,
		)
		return nil, perrors.WithCode(code.ErrTokenInvalid, "access token has been revoked")
	}

	if claims.SessionID == "" {
		return nil, perrors.WithCode(code.ErrTokenInvalid, "access token session is missing")
	}

	sess, err := s.sessionManager.Get(ctx, claims.SessionID)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to load session")
	}
	if sess == nil || !sess.IsActive() {
		return nil, perrors.WithCode(code.ErrTokenInvalid, "session has been revoked or expired")
	}

	decision, err := s.accessChecker.Evaluate(ctx, claims.UserID, claims.AccountID)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to evaluate subject access")
	}
	if !decision.IsAllowed() {
		return nil, subjectAccessVerifyError(decision.Status)
	}

	return claims, nil
}

func subjectAccessVerifyError(status sessiondomain.SubjectAccessStatus) error {
	switch status {
	case sessiondomain.SubjectAccessBlocked:
		return perrors.WithCode(code.ErrUserBlocked, "user is blocked")
	case sessiondomain.SubjectAccessDisabled:
		return perrors.WithCode(code.ErrCredentialDisabled, "account is disabled")
	case sessiondomain.SubjectAccessLocked:
		return perrors.WithCode(code.ErrCredentialLocked, "account is locked")
	default:
		return perrors.WithCode(code.ErrUserInactive, "user is inactive")
	}
}
