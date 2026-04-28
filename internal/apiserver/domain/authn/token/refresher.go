package token

import (
	"context"
	"fmt"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/security/sanitize"
)

// TokenRefresher 令牌刷新者
type TokenRefresher struct {
	tokenGenerator TokenGenerator // JWT 生成器
	tokenStore     TokenStore     // 令牌存储（Redis）
	sessionManager SessionManager
	accessChecker  SubjectAccessEvaluator
	accessTTL      time.Duration // 访问令牌有效期
	refreshTTL     time.Duration // 刷新令牌有效期
}

// NewTokenRefresher 创建令牌刷新者
func NewTokenRefresher(
	tokenGenerator TokenGenerator,
	tokenStore TokenStore,
	sessionManager SessionManager,
	accessChecker SubjectAccessEvaluator,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) *TokenRefresher {
	return &TokenRefresher{
		tokenGenerator: tokenGenerator,
		tokenStore:     tokenStore,
		sessionManager: sessionManager,
		accessChecker:  accessChecker,
		accessTTL:      accessTTL,
		refreshTTL:     refreshTTL,
	}
}

// RefreshToken 刷新令牌
func (s *TokenRefresher) RefreshToken(ctx context.Context, refreshTokenValue string) (*TokenPair, error) {
	l := logger.L(ctx)

	l.Debugw("开始刷新令牌",
		"action", "refresh",
		"resource", "refresh_token",
	)

	// 从 Redis 获取刷新令牌
	refreshToken, err := s.tokenStore.GetRefreshToken(ctx, refreshTokenValue)
	if err != nil {
		l.Warnw("从存储加载刷新令牌失败",
			"action", "refresh",
			"resource", "refresh_token",
			"error", err.Error(),
			"token_hint", sanitize.MaskToken(refreshTokenValue),
		)
		log.Warnw("failed to load refresh token from store",
			"error", err,
			"token_hint", sanitize.MaskToken(refreshTokenValue),
		)
		return nil, perrors.WrapC(err, code.ErrTokenInvalid, "refresh token not found or invalid")
	}

	if refreshToken == nil {
		l.Warnw("刷新令牌在存储中不存在",
			"action", "refresh",
			"resource", "refresh_token",
			"token_hint", sanitize.MaskToken(refreshTokenValue),
		)
		log.Warnw("refresh token not found in store",
			"token_hint", sanitize.MaskToken(refreshTokenValue),
		)
		return nil, perrors.WithCode(code.ErrTokenInvalid, "refresh token not found")
	}

	sess, err := s.sessionManager.Get(ctx, refreshToken.SessionID)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to load session")
	}
	if sess == nil || !sess.IsActive() {
		return nil, perrors.WithCode(code.ErrTokenInvalid, "session has been revoked or expired")
	}

	decision, err := s.accessChecker.Evaluate(ctx, refreshToken.UserID, refreshToken.AccountID)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to evaluate subject access")
	}
	if !decision.IsAllowed() {
		return nil, subjectAccessError(decision.Status)
	}

	// 检查刷新令牌是否过期
	if refreshToken.IsExpired() {
		l.Warnw("刷新令牌已过期",
			"action", "refresh",
			"resource", "refresh_token",
			"token_hint", sanitize.MaskToken(refreshTokenValue),
			"user_id", refreshToken.UserID.String(),
		)
		log.Infow("refresh token expired",
			"token_hint", sanitize.MaskToken(refreshTokenValue),
			"user_id", refreshToken.UserID,
		)
		// 删除过期的刷新令牌
		_ = s.tokenStore.DeleteRefreshToken(ctx, refreshTokenValue)
		return nil, perrors.WithCode(code.ErrExpired, "refresh token has expired")
	}

	amr := refreshToken.AMR
	if len(amr) == 0 {
		amr = []string{string(authentication.AMRJWTToken)}
	}
	claims := authentication.ClaimsFromStringMap(refreshToken.SessionClaims)
	if claims == nil {
		claims = make(map[string]any)
	}

	l.Debugw("刷新令牌有效，准备颁发新令牌",
		"action", "refresh",
		"resource", "refresh_token",
		"user_id", refreshToken.UserID.String(),
		"account_id", refreshToken.AccountID.String(),
		"tenant_id", refreshToken.TenantID.String(),
		"amr", amr,
		"claims", claims,
	)

	// 创建新的认证主体（从刷新令牌中恢复）
	principal := &authentication.Principal{
		UserID:    refreshToken.UserID,
		AccountID: refreshToken.AccountID,
		TenantID:  refreshToken.TenantID,
		SessionID: refreshToken.SessionID,
		AMR:       amr,
		Claims:    claims,
	}

	// 颁发新的令牌对
	l.Debugw("通过颁发者创建新的令牌对",
		"action", "refresh",
		"resource", "token",
		"principal", fmt.Sprintf("%+v", principal),
		"access_ttl", s.accessTTL.Seconds(),
		"refresh_ttl", s.refreshTTL.Seconds(),
	)

	newTokenPair, err := NewTokenIssuer(s.tokenGenerator, s.tokenStore, s.sessionManager, s.accessTTL, s.refreshTTL).issueTokenPair(ctx, principal, sess)
	if err != nil {
		l.Errorw("颁发新令牌对失败",
			"action", "refresh",
			"resource", "token",
			"error", err.Error(),
		)
		return nil, err
	}

	// 轮换刷新令牌：删除旧的刷新令牌
	if err := s.tokenStore.DeleteRefreshToken(ctx, refreshTokenValue); err != nil {
		// 记录错误但不影响主流程
		log.Errorw("failed to delete stale refresh token after rotation",
			"error", err,
			"token_hint", sanitize.MaskToken(refreshTokenValue),
		)
	}

	if err := s.sessionManager.Extend(ctx, sess.SessionID, newTokenPair.RefreshToken.ExpiresAt); err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to extend session ttl")
	}

	return newTokenPair, nil
}

// RevokeRefreshToken 撤销刷新令牌
func (s *TokenRefresher) RevokeRefreshToken(ctx context.Context, refreshTokenValue string) error {
	refreshToken, err := s.tokenStore.GetRefreshToken(ctx, refreshTokenValue)
	if err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to load refresh token")
	}
	if refreshToken != nil && refreshToken.SessionID != "" {
		if err := s.sessionManager.Revoke(ctx, refreshToken.SessionID, "refresh_token_revoked", refreshToken.UserID.String()); err != nil {
			return perrors.WrapC(err, code.ErrInternalServerError, "failed to revoke refresh token session")
		}
	}
	if err := s.tokenStore.DeleteRefreshToken(ctx, refreshTokenValue); err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to revoke refresh token")
	}
	return nil
}

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
