package token

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/google/uuid"
)

// sessionTokenPairIssuer 基于已存在的 session 签发 access token 并保存 refresh token。
type sessionTokenPairIssuer struct {
	tokenCodec     AccessTokenCodec      // 令牌编码器
	tokenStore     Store                 // 令牌存储
	refreshExpirer SessionRefreshExpirer // refresh token 过期时间计算器
	claimMapper    ClaimMapper           // 声明映射器
	accessTTL      time.Duration         // 令牌有效期
}

// 确保 sessionTokenPairIssuer 实现 sessionTokenPairIssuerPort 接口。
var _ sessionTokenPairIssuerPort = (*sessionTokenPairIssuer)(nil)

// newSessionTokenPairIssuer 创建 sessionTokenPairIssuer。
func newSessionTokenPairIssuer(
	tokenCodec AccessTokenCodec,
	tokenStore Store,
	refreshExpirer SessionRefreshExpirer,
	claimMapper ClaimMapper,
	accessTTL time.Duration,
) *sessionTokenPairIssuer {
	return &sessionTokenPairIssuer{
		tokenCodec:     tokenCodec,
		tokenStore:     tokenStore,
		refreshExpirer: refreshExpirer,
		claimMapper:    normalizeClaimMapper(claimMapper),
		accessTTL:      accessTTL,
	}
}

// IssueTokenPair 颁发令牌对。
func (s *sessionTokenPairIssuer) IssueTokenPair(ctx context.Context, principal *authentication.Principal, sess *sessiondomain.Session) (*TokenPair, error) {
	if principal == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "principal is required")
	}
	if sess == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "session is required")
	}

	// 克隆认证主体声明
	claims := cloneAnyMap(principal.Claims)

	// 确保认证时间声明存在
	now := time.Now().UTC()
	ensureAuthTime(claims, now)

	// 颁发访问令牌
	accessToken, err := s.tokenCodec.IssueAccessToken(ctx, &Principal{
		UserID:          principal.UserID,
		LoginIdentityID: principal.LoginIdentityID,
		SessionID:       sess.SessionID,
		AuthMethod:      principal.AuthMethod,
		Realm:           principal.Realm,
		AMR:             append([]string(nil), principal.AMR...),
		Claims:          claims,
	}, s.accessTTL)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate access token")
	}

	// 颁发刷新令牌
	refreshToken, err := s.issueRefreshToken(ctx, principal, sess, claims, now)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate refresh token")
	}

	// 返回令牌对
	return NewTokenPair(accessToken, refreshToken), nil
}

// issueRefreshToken 颁发刷新令牌。
func (s *sessionTokenPairIssuer) issueRefreshToken(ctx context.Context, principal *authentication.Principal, sess *sessiondomain.Session, claims map[string]any, now time.Time) (*Token, error) {
	// 计算下一次 refresh token 的过期时间
	refreshExpiresAt, err := s.refreshExpirer.NextRefreshExpiresAt(now, sess)
	if err != nil {
		return nil, err
	}
	// 创建刷新令牌
	refreshToken := NewRefreshTokenWithExpiry(
		uuid.New().String(),
		uuid.New().String(),
		sess.SessionID,
		principal.UserID,
		principal.LoginIdentityID,
		meta.ZeroID,
		principal.AMR,
		s.claimMapper.Encode(claims),
		refreshExpiresAt,
	)

	// 设置颁发时间
	refreshToken.IssuedAt = now
	// 设置认证方法
	refreshToken.AuthMethod = principal.AuthMethod
	// 设置认证域
	refreshToken.Realm = principal.Realm

	// 保存刷新令牌到存储
	if err := s.tokenStore.SaveRefreshToken(ctx, refreshToken); err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to save refresh token")
	}

	return refreshToken, nil
}

// ensureAuthTime 确保认证时间声明存在。
func ensureAuthTime(claims map[string]any, now time.Time) {
	if claims == nil {
		return
	}
	if value, ok := claims["auth_time"]; ok && value != nil {
		if text, ok := value.(string); ok && text != "" {
			return
		}
		if t, ok := value.(time.Time); ok && !t.IsZero() {
			claims["auth_time"] = t.UTC().Format(time.RFC3339)
			return
		}
	}
	claims["auth_time"] = now.Format(time.RFC3339)
}

// cloneAnyMap 克隆任意映射。
func cloneAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
