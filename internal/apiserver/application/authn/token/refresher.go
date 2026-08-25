package token

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	admissionapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// refresher 用于根据 refresh token 刷新 access token 和 refresh token。
type refresher struct {
	tokenPairMinter    tokenPairMinterPort
	tokenStore         Store
	sessionLoader      SessionLoader
	sessionRevoker     SessionRevoker
	sessionExtender    SessionExtender
	admissionPolicy    AdmissionPolicy
	refreshClaimsCodec RefreshClaimsCodec
}

// 确保 refresher 实现 refresherPort 接口。
var _ refresherPort = (*refresher)(nil)

// newRefresher 创建 refresher。
func newRefresher(
	tokenPairMinter tokenPairMinterPort,
	tokenStore Store,
	sessionLoader SessionLoader,
	sessionRevoker SessionRevoker,
	sessionExtender SessionExtender,
	admissionPolicy AdmissionPolicy,
	refreshClaimsCodec RefreshClaimsCodec,
) refresherPort {
	return &refresher{
		tokenPairMinter:    tokenPairMinter,
		tokenStore:         tokenStore,
		sessionLoader:      sessionLoader,
		sessionRevoker:     sessionRevoker,
		sessionExtender:    sessionExtender,
		admissionPolicy:    admissionPolicy,
		refreshClaimsCodec: normalizeRefreshClaimsCodec(refreshClaimsCodec),
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

	// 确保认证主体满足准入策略
	if err := s.requireAdmission(ctx, refreshToken); err != nil {
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

	if err := s.sessionExtender.ExtendToRefreshExpiry(ctx, sess, newTokenPair.RefreshToken.ExpiresAt); err != nil {
		if perrors.IsCode(err, code.ErrSessionInactive) || perrors.IsCode(err, code.ErrInvalidArgument) {
			return nil, err
		}
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to extend session ttl")
	}

	rotated, err := s.tokenStore.RotateRefreshToken(ctx, refreshTokenValue, refreshToken.ID, newTokenPair.RefreshToken)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to rotate refresh token")
	}
	if !rotated {
		logger.L(ctx).Infow("refresh token rotation rejected because the old token was already consumed",
			"action", logger.ActionRefresh,
			"resource", logger.ResourceToken,
			"token_type", "refresh",
			"result", "conflict",
		)
		if err := s.revokeReplaySession(ctx, refreshToken.SessionID, refreshToken.UserID.String()); err != nil {
			return nil, err
		}
		return nil, perrors.WithCode(code.ErrRefreshTokenNotFound, "refresh token not found")
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
		if err := s.sessionRevoker.Revoke(ctx, refreshToken.SessionID, "refresh_token_revoked", refreshToken.UserID.String()); err != nil {
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
		consumed, consumedErr := s.tokenStore.GetConsumedRefreshToken(ctx, refreshTokenValue)
		if consumedErr != nil {
			return nil, perrors.WrapC(consumedErr, code.ErrInternalServerError, "failed to inspect consumed refresh token")
		}
		if consumed != nil {
			if err := s.revokeReplaySession(ctx, consumed.SessionID, consumed.UserID.String()); err != nil {
				return nil, err
			}
		}
		return nil, perrors.WithCode(code.ErrRefreshTokenNotFound, "refresh token not found")
	}
	return refreshToken, nil
}

func (s *refresher) revokeReplaySession(ctx context.Context, sessionID, userID string) error {
	if sessionID == "" {
		return perrors.WithCode(code.ErrInternalServerError, "consumed refresh token has no session")
	}
	if err := s.sessionRevoker.Revoke(ctx, sessionID, "refresh_token_replay", userID); err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to revoke replayed refresh token session")
	}
	logger.L(ctx).Infow("refresh token replay revoked session",
		"action", logger.ActionRevoke,
		"resource", logger.ResourceSession,
		"result", "success",
	)
	return nil
}

// loadActiveSession 加载活跃会话
func (s *refresher) loadActiveSession(ctx context.Context, sessionID string) (*sessiondomain.Session, error) {
	sess, err := s.sessionLoader.GetActive(ctx, sessionID)
	if err != nil {
		if perrors.IsCode(err, code.ErrSessionInactive) {
			return nil, err
		}
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to load session")
	}
	return sess, nil
}

// requireAdmission 确保认证主体满足准入策略。
func (s *refresher) requireAdmission(ctx context.Context, refreshToken *Token) error {
	return admissionapp.Require(ctx, s.admissionPolicy, refreshToken.UserID, refreshToken.LoginIdentityID)
}

// ensureRefreshTokenUsable 确保刷新令牌可用
func (s *refresher) ensureRefreshTokenUsable(ctx context.Context, refreshTokenValue string, refreshToken *Token) error {
	if !refreshToken.IsExpired() {
		return nil
	}
	_ = s.tokenStore.DeleteRefreshToken(ctx, refreshTokenValue)
	return perrors.WithCode(code.ErrRefreshTokenExpired, "refresh token has expired")
}

// principalFromRefreshToken 从刷新令牌创建主体
func (s *refresher) principalFromRefreshToken(refreshToken *Token) *authentication.Principal {
	amr := refreshToken.AMR
	if len(amr) == 0 {
		amr = []string{"jwt"}
	}
	claims := s.refreshClaimsCodec.Decode(refreshToken.SessionClaims)
	if claims == nil {
		claims = make(map[string]any)
	}
	return &authentication.Principal{
		UserID:          refreshToken.UserID,
		LoginIdentityID: refreshToken.LoginIdentityID,
		TenantID:        refreshToken.TenantID,
		SessionID:       refreshToken.SessionID,
		AuthMethod:      refreshToken.AuthMethod,
		Realm:           refreshToken.Realm,
		AMR:             amr,
		Claims:          claims,
	}
}

// issueRotatedTokenPair 颁发新的令牌对
func (s *refresher) issueRotatedTokenPair(ctx context.Context, principal *authentication.Principal, sess *sessiondomain.Session) (*TokenPair, error) {
	if s.tokenPairMinter == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "token pair minter is not configured")
	}
	newTokenPair, err := s.tokenPairMinter.MintTokenPair(ctx, principal, sess)
	if err != nil {
		return nil, err
	}
	if newTokenPair == nil || newTokenPair.AccessToken == nil || newTokenPair.RefreshToken == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "token pair minter returned incomplete token pair")
	}
	return newTokenPair, nil
}
