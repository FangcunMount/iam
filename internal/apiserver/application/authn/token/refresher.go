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

type refresher struct {
	issuer        accessPairIssuer
	tokenStore    Store
	sessionManger SessionManager
	accessChecker SubjectAccessEvaluator
	claimMapper   ClaimMapper
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

type accessPairIssuer interface {
	IssueTokenPair(ctx context.Context, principal *authentication.Principal, sess *sessiondomain.Session) (*TokenPair, error)
}

func NewRefresher(
	tokenIssuer accessPairIssuer,
	tokenStore Store,
	sessionManager SessionManager,
	accessChecker SubjectAccessEvaluator,
	claimMapper ClaimMapper,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) Refresher {
	return &refresher{
		issuer:        tokenIssuer,
		tokenStore:    tokenStore,
		sessionManger: sessionManager,
		accessChecker: accessChecker,
		claimMapper:   normalizeClaimMapper(claimMapper),
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}
}

func (s *refresher) RefreshToken(ctx context.Context, refreshTokenValue string) (*TokenPair, error) {
	l := logger.L(ctx)
	l.Debugw("开始刷新令牌", "action", "refresh", "resource", "refresh_token")

	refreshToken, err := s.tokenStore.GetRefreshToken(ctx, refreshTokenValue)
	if err != nil {
		log.Warnw("failed to load refresh token from store", "error", err, "token_hint", sanitize.MaskToken(refreshTokenValue))
		return nil, perrors.WrapC(err, code.ErrTokenInvalid, "refresh token not found or invalid")
	}
	if refreshToken == nil {
		return nil, perrors.WithCode(code.ErrTokenInvalid, "refresh token not found")
	}

	sess, err := s.sessionManger.Get(ctx, refreshToken.SessionID)
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

	if refreshToken.IsExpired() {
		_ = s.tokenStore.DeleteRefreshToken(ctx, refreshTokenValue)
		return nil, perrors.WithCode(code.ErrExpired, "refresh token has expired")
	}

	amr := refreshToken.AMR
	if len(amr) == 0 {
		amr = []string{"jwt"}
	}
	claims := s.claimMapper.Decode(refreshToken.SessionClaims)
	if claims == nil {
		claims = make(map[string]any)
	}
	principal := &authentication.Principal{
		UserID:    refreshToken.UserID,
		AccountID: refreshToken.AccountID,
		TenantID:  refreshToken.TenantID,
		SessionID: refreshToken.SessionID,
		AMR:       amr,
		Claims:    claims,
	}

	l.Debugw("通过颁发者创建新的令牌对",
		"action", "refresh",
		"resource", "token",
		"principal", fmt.Sprintf("%+v", principal),
		"access_ttl", s.accessTTL.Seconds(),
		"refresh_ttl", s.refreshTTL.Seconds(),
	)
	if s.issuer == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "token issuer is not configured")
	}
	newTokenPair, err := s.issuer.IssueTokenPair(ctx, principal, sess)
	if err != nil {
		return nil, err
	}

	if err := s.tokenStore.DeleteRefreshToken(ctx, refreshTokenValue); err != nil {
		log.Errorw("failed to delete stale refresh token after rotation", "error", err, "token_hint", sanitize.MaskToken(refreshTokenValue))
	}
	if err := s.sessionManger.Extend(ctx, sess.SessionID, newTokenPair.RefreshToken.ExpiresAt); err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to extend session ttl")
	}
	return newTokenPair, nil
}

func (s *refresher) RevokeRefreshToken(ctx context.Context, refreshTokenValue string) error {
	refreshToken, err := s.tokenStore.GetRefreshToken(ctx, refreshTokenValue)
	if err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to load refresh token")
	}
	if refreshToken != nil && refreshToken.SessionID != "" {
		if err := s.sessionManger.Revoke(ctx, refreshToken.SessionID, "refresh_token_revoked", refreshToken.UserID.String()); err != nil {
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
