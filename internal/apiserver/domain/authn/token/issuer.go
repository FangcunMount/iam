package token

import (
	"context"
	"fmt"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/google/uuid"
)

// TokenIssuer 令牌颁发者
type TokenIssuer struct {
	tokenGenerator TokenGenerator // JWT 生成器
	tokenStore     TokenStore     // 令牌存储（Redis）
	sessionManager SessionManager // 会话管理器
	accessTTL      time.Duration  // 访问令牌有效期
	refreshTTL     time.Duration  // 刷新令牌有效期
}

// NewTokenIssuer 创建令牌颁发者
func NewTokenIssuer(
	tokenGenerator TokenGenerator,
	tokenStore TokenStore,
	sessionManager SessionManager,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) *TokenIssuer {
	return &TokenIssuer{
		tokenGenerator: tokenGenerator,
		tokenStore:     tokenStore,
		sessionManager: sessionManager,
		accessTTL:      accessTTL,
		refreshTTL:     refreshTTL,
	}
}

// IssueToken 颁发令牌对
func (s *TokenIssuer) IssueToken(ctx context.Context, principal *authentication.Principal) (*TokenPair, error) {
	l := logger.L(ctx)

	if principal == nil {
		l.Errorw("颁发令牌时 principal 为空",
			"action", logger.ActionCreate,
			"resource", "token",
		)
		return nil, perrors.WithCode(code.ErrInvalidArgument, "principal is required")
	}

	l.Debugw("开始颁发令牌对",
		"action", logger.ActionCreate,
		"resource", "token",
		"user_id", principal.UserID.String(),
		"account_id", principal.AccountID.String(),
		"tenant_id", principal.TenantID.String(),
		"amr", principal.AMR,
		"claims", principal.Claims,
	)

	// 登录成功后先创建会话，再基于 sid 签发 access / refresh token。
	sessionExpiresAt := time.Now().Add(s.refreshTTL)
	sess, err := s.sessionManager.Create(ctx, principal, sessionExpiresAt)
	if err != nil {
		l.Errorw("创建认证会话失败",
			"action", logger.ActionCreate,
			"resource", "session",
			"error", err.Error(),
		)
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to create session")
	}

	return s.issueTokenPair(ctx, principal, sess)
}

func (s *TokenIssuer) issueTokenPair(ctx context.Context, principal *authentication.Principal, sess *sessiondomain.Session) (*TokenPair, error) {
	l := logger.L(ctx)

	if sess == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "session is required")
	}

	principalWithSession := &authentication.Principal{
		UserID:    principal.UserID,
		AccountID: principal.AccountID,
		TenantID:  principal.TenantID,
		SessionID: sess.SessionID,
		AMR:       append([]string(nil), principal.AMR...),
		Claims:    cloneAnyMap(principal.Claims),
	}

	// 生成访问令牌（JWT）
	l.Debugw("生成访问令牌",
		"action", logger.ActionCreate,
		"resource", "access_token",
		"ttl_seconds", s.accessTTL.Seconds(),
		"principal", fmt.Sprintf("%+v", principalWithSession),
	)

	accessToken, err := s.tokenGenerator.GenerateAccessToken(ctx, principalWithSession, s.accessTTL)
	if err != nil {
		l.Errorw("访问令牌生成失败",
			"action", logger.ActionCreate,
			"resource", "access_token",
			"error", err.Error(),
		)
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate access token")
	}

	l.Debugw("访问令牌生成成功",
		"action", logger.ActionCreate,
		"resource", "access_token",
		"token_id", accessToken.ID,
		"session_id", sess.SessionID,
	)

	// 生成刷新令牌（UUID）
	l.Debugw("生成刷新令牌",
		"action", logger.ActionCreate,
		"resource", "refresh_token",
		"ttl_seconds", s.refreshTTL.Seconds(),
		"principal", fmt.Sprintf("%+v", principal),
	)

	refreshTokenValue := uuid.New().String()
	sessionClaims := authentication.FlattenClaimsForJWT(principal.Claims)
	refreshToken := NewRefreshToken(
		uuid.New().String(), // token ID
		refreshTokenValue,   // token value
		sess.SessionID,
		principal.UserID, // user ID
		principal.AccountID,
		principal.TenantID,
		principal.AMR,
		sessionClaims,
		s.refreshTTL,
	)

	// 保存刷新令牌到 Redis
	l.Debugw("保存刷新令牌到存储",
		"action", logger.ActionCreate,
		"resource", "refresh_token",
		"refresh_token_id", refreshToken.ID,
		"principal", fmt.Sprintf("%+v", principal),
	)

	if err := s.tokenStore.SaveRefreshToken(ctx, refreshToken); err != nil {
		l.Errorw("刷新令牌保存失败",
			"action", logger.ActionCreate,
			"resource", "refresh_token",
			"error", err.Error(),
			"principal", fmt.Sprintf("%+v", principal),
		)
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to save refresh token")
	}

	l.Debugw("令牌对颁发成功",
		"action", logger.ActionCreate,
		"resource", "token",
		"user_id", principal.UserID.String(),
		"session_id", sess.SessionID,
		"access_token_id", accessToken.ID,
		"refresh_token_id", refreshToken.ID,
		"result", logger.ResultSuccess,
		"principal", fmt.Sprintf("%+v", principal),
	)

	return NewTokenPair(accessToken, refreshToken), nil
}

// IssueServiceToken 签发服务间访问令牌。
func (s *TokenIssuer) IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*TokenPair, error) {
	l := logger.L(ctx)

	if subject == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "subject is required")
	}

	if ttl <= 0 {
		ttl = s.accessTTL
	}

	l.Debugw("开始签发服务令牌",
		"action", logger.ActionCreate,
		"resource", "service_token",
		"subject", subject,
		"audience", audience,
		"ttl_seconds", ttl.Seconds(),
	)

	serviceToken, err := s.tokenGenerator.GenerateServiceToken(ctx, subject, audience, attributes, ttl)
	if err != nil {
		l.Errorw("服务令牌生成失败",
			"action", logger.ActionCreate,
			"resource", "service_token",
			"subject", subject,
			"error", err.Error(),
		)
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate service token")
	}

	l.Debugw("服务令牌签发成功",
		"action", logger.ActionCreate,
		"resource", "service_token",
		"subject", subject,
		"token_id", serviceToken.ID,
		"result", logger.ResultSuccess,
	)

	return NewTokenPair(serviceToken, nil), nil
}

// RevokeAccessToken 撤销访问令牌
//
// 将访问令牌标记为已撤销，并联动撤销其所属会话。
func (s *TokenIssuer) RevokeAccessToken(ctx context.Context, tokenValue string) error {
	l := logger.L(ctx)

	l.Debugw("开始撤销访问令牌",
		"action", logger.ActionDelete,
		"resource", "access_token",
	)

	// 解析令牌以获取 tokenID 和过期时间
	claims, err := s.tokenGenerator.ParseAccessToken(ctx, tokenValue)
	if err != nil {
		l.Warnw("令牌解析失败",
			"action", logger.ActionDelete,
			"resource", "access_token",
			"error", err.Error(),
		)
		return perrors.WrapC(err, code.ErrTokenInvalid, "failed to parse token for revocation")
	}

	// 如果令牌已过期，无需写入撤销标记
	if claims.IsExpired() {
		l.Debugw("令牌已过期，无需写入撤销标记",
			"action", logger.ActionDelete,
			"resource", "access_token",
			"token_id", claims.TokenID,
		)
		return nil
	}

	// 写入访问令牌撤销标记，TTL 设置为剩余有效期
	expiry := time.Until(claims.ExpiresAt)
	if expiry <= 0 {
		l.Debugw("令牌已过期，无需写入撤销标记",
			"action", logger.ActionDelete,
			"resource", "access_token",
			"token_id", claims.TokenID,
		)
		return nil // 已过期
	}

	l.Debugw("写入访问令牌撤销标记",
		"action", logger.ActionDelete,
		"resource", "access_token",
		"token_id", claims.TokenID,
		"remaining_ttl_seconds", expiry.Seconds(),
	)

	if err := s.tokenStore.MarkAccessTokenRevoked(ctx, claims.TokenID, expiry); err != nil {
		l.Errorw("访问令牌撤销标记写入失败",
			"action", logger.ActionDelete,
			"resource", "access_token",
			"token_id", claims.TokenID,
			"error", err.Error(),
		)
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to mark access token revoked")
	}

	if claims.SessionID != "" {
		if err := s.sessionManager.Revoke(ctx, claims.SessionID, "access_token_revoked", claims.Subject); err != nil {
			l.Errorw("访问令牌所属会话撤销失败",
				"action", logger.ActionDelete,
				"resource", "session",
				"session_id", claims.SessionID,
				"error", err.Error(),
			)
			return perrors.WrapC(err, code.ErrInternalServerError, "failed to revoke token session")
		}
	}

	l.Debugw("访问令牌撤销成功",
		"action", logger.ActionDelete,
		"resource", "access_token",
		"token_id", claims.TokenID,
		"result", logger.ResultSuccess,
	)

	return nil
}

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
