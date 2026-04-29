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

type issuer struct {
	tokenCodec    AccessTokenCodec
	tokenStore    Store
	sessionManger SessionManager
	claimMapper   ClaimMapper
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

func NewIssuer(
	tokenCodec AccessTokenCodec,
	tokenStore Store,
	sessionManager SessionManager,
	claimMapper ClaimMapper,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) Issuer {
	return &issuer{
		tokenCodec:    tokenCodec,
		tokenStore:    tokenStore,
		sessionManger: sessionManager,
		claimMapper:   normalizeClaimMapper(claimMapper),
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}
}

func (s *issuer) IssueToken(ctx context.Context, principal *authentication.Principal) (*TokenPair, error) {
	l := logger.L(ctx)
	if principal == nil {
		l.Errorw("颁发令牌时 principal 为空", "action", logger.ActionCreate, "resource", "token")
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

	sessionExpiresAt := time.Now().Add(s.refreshTTL)
	sess, err := s.sessionManger.Create(ctx, principal, sessionExpiresAt)
	if err != nil {
		l.Errorw("创建认证会话失败", "action", logger.ActionCreate, "resource", "session", "error", err.Error())
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to create session")
	}

	return s.issueTokenPair(ctx, principal, sess)
}

func (s *issuer) issueTokenPair(ctx context.Context, principal *authentication.Principal, sess *sessiondomain.Session) (*TokenPair, error) {
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

	l.Debugw("生成访问令牌",
		"action", logger.ActionCreate,
		"resource", "access_token",
		"ttl_seconds", s.accessTTL.Seconds(),
		"principal", fmt.Sprintf("%+v", principalWithSession),
	)
	accessToken, err := s.tokenCodec.IssueAccessToken(ctx, principalWithSession, s.accessTTL)
	if err != nil {
		l.Errorw("访问令牌生成失败", "action", logger.ActionCreate, "resource", "access_token", "error", err.Error())
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate access token")
	}

	refreshTokenValue := uuid.New().String()
	refreshToken := NewRefreshToken(
		uuid.New().String(),
		refreshTokenValue,
		sess.SessionID,
		principal.UserID,
		principal.AccountID,
		principal.TenantID,
		principal.AMR,
		s.claimMapper.Encode(principal.Claims),
		s.refreshTTL,
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
	)
	return NewTokenPair(accessToken, refreshToken), nil
}

func (s *issuer) IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*TokenPair, error) {
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
	serviceToken, err := s.tokenCodec.IssueServiceToken(ctx, subject, audience, attributes, ttl)
	if err != nil {
		l.Errorw("服务令牌生成失败", "action", logger.ActionCreate, "resource", "service_token", "subject", subject, "error", err.Error())
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate service token")
	}
	return NewTokenPair(serviceToken, nil), nil
}

func (s *issuer) RevokeAccessToken(ctx context.Context, tokenValue string) error {
	l := logger.L(ctx)
	l.Debugw("开始撤销访问令牌", "action", logger.ActionDelete, "resource", "access_token")

	claims, err := s.tokenCodec.VerifyAccessToken(ctx, tokenValue)
	if err != nil {
		l.Warnw("令牌解析失败", "action", logger.ActionDelete, "resource", "access_token", "error", err.Error())
		return perrors.WrapC(err, code.ErrTokenInvalid, "failed to parse token for revocation")
	}
	if claims.IsExpired() {
		l.Debugw("令牌已过期，无需写入撤销标记", "action", logger.ActionDelete, "resource", "access_token", "token_id", claims.TokenID)
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
		if err := s.sessionManger.Revoke(ctx, claims.SessionID, "access_token_revoked", claims.Subject); err != nil {
			return perrors.WrapC(err, code.ErrInternalServerError, "failed to revoke token session")
		}
	}
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
