package token

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/google/uuid"
)

// tokenSetMinter 是用户令牌颁发器的实现。
type tokenSetMinter struct {
	tokenCodec         AccessTokenCodec
	refreshExpirer     SessionRefreshExpirer
	refreshClaimsCodec RefreshClaimsCodec
	accessTTL          time.Duration
}

// newTokenSetMinter 创建用户令牌颁发器。
func newTokenSetMinter(tokenCodec AccessTokenCodec, refreshExpirer SessionRefreshExpirer, refreshClaimsCodec RefreshClaimsCodec, accessTTL time.Duration) TokenSetMinter {
	return &tokenSetMinter{
		tokenCodec: tokenCodec, refreshExpirer: refreshExpirer,
		refreshClaimsCodec: normalizeRefreshClaimsCodec(refreshClaimsCodec), accessTTL: accessTTL,
	}
}

// MintTokenSet 颁发用户令牌。
func (s *tokenSetMinter) MintTokenSet(ctx context.Context, principal *authentication.Principal, sess *sessiondomain.Session) (*UserTokenSet, error) {
	if principal == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "principal is required")
	}
	if sess == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "session is required")
	}
	if err := validatePrincipalSessionAlignment(principal, sess); err != nil {
		return nil, err
	}

	// 构建用户令牌主体
	subject := accessTokenSubjectFromAuth(principal, sess)
	claims := cloneAnyMap(subject.Claims)
	now := time.Now().UTC()
	// 确保认证时间
	ensureAuthTime(claims, now)

	// 颁发访问令牌
	accessToken, err := s.tokenCodec.IssueAccessToken(ctx, &AccessTokenSubject{
		UserID: subject.UserID, LoginIdentityID: subject.LoginIdentityID, SessionID: subject.SessionID,
		TenantID: subject.TenantID, AuthMethod: subject.AuthMethod, Realm: subject.Realm,
		AMR: append([]string(nil), subject.AMR...), Claims: claims,
	}, s.accessTTL)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate access token")
	}

	// 颁发刷新令牌
	refreshToken, err := s.issueRefreshToken(subject, sess, claims, now)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate refresh token")
	}
	return NewUserTokenSet(accessToken, refreshToken), nil
}

// issueRefreshToken 颁发刷新令牌。
func (s *tokenSetMinter) issueRefreshToken(subject *AccessTokenSubject, sess *sessiondomain.Session, claims map[string]any, now time.Time) (*RefreshToken, error) {
	refreshExpiresAt, err := s.refreshExpirer.NextRefreshExpiresAt(now, sess)
	if err != nil {
		return nil, err
	}
	token := NewRefreshTokenWithExpiry(
		uuid.NewString(), uuid.NewString(), sess.SessionID, subject.UserID, subject.LoginIdentityID,
		subject.TenantID, subject.AMR, s.refreshClaimsCodec.Encode(claims), refreshExpiresAt,
	)
	token.IssuedAt = now
	token.AuthMethod = subject.AuthMethod
	token.Realm = subject.Realm
	return token, nil
}

// ensureAuthTime 确保认证时间。
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

// serviceTokenIssuer 是服务令牌颁发器的实现。
type serviceTokenIssuer struct {
	tokenCodec AccessTokenCodec
	accessTTL  time.Duration
}

// newServiceTokenIssuer 创建服务令牌颁发器。
func newServiceTokenIssuer(tokenCodec AccessTokenCodec, accessTTL time.Duration) ServiceTokenIssuer {
	return &serviceTokenIssuer{tokenCodec: tokenCodec, accessTTL: accessTTL}
}

// IssueServiceToken 颁发服务令牌。
func (s *serviceTokenIssuer) IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*ServiceToken, error) {
	// 验证主题
	if subject == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "subject is required")
	}
	// 验证 TTL
	if ttl <= 0 {
		ttl = s.accessTTL
	}
	token, err := s.tokenCodec.IssueServiceToken(ctx, subject, audience, attributes, ttl)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate service token")
	}
	return token, nil
}
