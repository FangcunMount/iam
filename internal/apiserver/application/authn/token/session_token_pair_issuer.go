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

// sessionTokenPairIssuerPort 基于已存在的 session 签发 access token 并保存 refresh token。
//
// Login 会先创建 session 再调用该组件；Refresh 会复用已有 session 后调用该组件。
type sessionTokenPairIssuerPort interface {
	// IssueTokenPair 根据认证主体和会话信息签发 access token，并保存新的 refresh token。
	// 返回值必须包含 access token 和 refresh token。
	IssueTokenPair(ctx context.Context, principal *authentication.Principal, sess *sessiondomain.Session) (*TokenPair, error)
}

// sessionTokenPairIssuer 基于已存在的 session 签发 access token 并保存 refresh token。
type sessionTokenPairIssuer struct {
	tokenCodec  AccessTokenCodec // 令牌编码器
	tokenStore  Store            // 令牌存储
	claimMapper ClaimMapper      // 声明映射器
	accessTTL   time.Duration    // 令牌有效期
	refreshTTL  time.Duration    // 刷新令牌有效期
}

// 确保 sessionTokenPairIssuer 实现 sessionTokenPairIssuerPort 接口。
var _ sessionTokenPairIssuerPort = (*sessionTokenPairIssuer)(nil)

// newSessionTokenPairIssuer 创建 sessionTokenPairIssuer。
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

// IssueTokenPair 颁发令牌对。
func (s *sessionTokenPairIssuer) IssueTokenPair(ctx context.Context, principal *authentication.Principal, sess *sessiondomain.Session) (*TokenPair, error) {
	if principal == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "principal is required")
	}
	if sess == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "session is required")
	}
	now := time.Now().UTC()
	claims := cloneAnyMap(principal.Claims)
	ensureAuthTime(claims, now)

	// 创建主体与会话信息
	principalWithSession := &Principal{
		UserID:          principal.UserID,
		LoginIdentityID: principal.LoginIdentityID,
		TenantID:        principal.TenantID,
		SessionID:       sess.SessionID,
		AuthMethod:      principal.AuthMethod,
		Realm:           principal.Realm,
		AMR:             append([]string(nil), principal.AMR...),
		Claims:          claims,
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
		principal.LoginIdentityID,
		principal.TenantID,
		principal.AMR,
		s.claimMapper.Encode(claims),
		s.refreshTTL,
	)
	refreshToken.AuthMethod = principal.AuthMethod
	refreshToken.Realm = principal.Realm

	// 保存刷新令牌
	if err := s.tokenStore.SaveRefreshToken(ctx, refreshToken); err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to save refresh token")
	}

	// 返回令牌对
	return NewTokenPair(accessToken, refreshToken), nil
}

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

func cloneAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
