package token

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/google/uuid"
)

// sessionTokenPairIssuer  基于已存在的 session 签发 access token 并保存 refresh token。
type sessionTokenPairIssuer struct {
	tokenCodec  AccessTokenCodec
	tokenStore  Store
	claimMapper ClaimMapper
	accessTTL   time.Duration
	refreshTTL  time.Duration
}

// 确保 sessionTokenPairIssuer 实现 SessionTokenPairIssuer 接口
var _ SessionTokenPairIssuer = (*sessionTokenPairIssuer)(nil)

// newSessionTokenPairIssuer 创建 sessionTokenPairIssuer
func newSessionTokenPairIssuer(
	tokenCodec AccessTokenCodec,
	tokenStore Store,
	claimMapper ClaimMapper,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) *sessionTokenPairIssuer {
	return &sessionTokenPairIssuer{
		tokenCodec:  tokenCodec,
		tokenStore:  tokenStore,
		claimMapper: normalizeClaimMapper(claimMapper),
		accessTTL:   accessTTL,
		refreshTTL:  refreshTTL,
	}
}

// IssueTokenPair 颁发令牌对
func (s *sessionTokenPairIssuer) IssueTokenPair(ctx context.Context, principal *authentication.Principal, sess *sessiondomain.Session) (*TokenPair, error) {
	if principal == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "principal is required")
	}
	if sess == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "session is required")
	}

	// 创建主体与会话信息
	principalWithSession := &Principal{
		UserID:    principal.UserID,
		AccountID: principal.AccountID,
		TenantID:  principal.TenantID,
		SessionID: sess.SessionID,
		AMR:       append([]string(nil), principal.AMR...),
		Claims:    cloneAnyMap(principal.Claims),
	}

	// 颁发访问令牌
	accessToken, err := s.tokenCodec.IssueAccessToken(ctx, principalWithSession, s.accessTTL)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate access token")
	}

	// 颁发刷新令牌
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

	// 保存刷新令牌
	if err := s.tokenStore.SaveRefreshToken(ctx, refreshToken); err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to save refresh token")
	}

	// 返回令牌对
	return NewTokenPair(accessToken, refreshToken), nil
}
