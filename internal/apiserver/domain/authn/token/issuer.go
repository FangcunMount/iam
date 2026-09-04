package token

import (
	"context"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/google/uuid"
)

// tokenSetMinter 是用户令牌颁发器的实现。
type tokenSetMinter struct {
	tokenCodec     BearerTokenCodec
	refreshExpirer SessionRefreshExpirer
	accessTTL      time.Duration
}

// newTokenSetMinter 创建用户令牌颁发器。
func newTokenSetMinter(tokenCodec BearerTokenCodec, refreshExpirer SessionRefreshExpirer, accessTTL time.Duration) TokenSetMinter {
	return &tokenSetMinter{
		tokenCodec: tokenCodec, refreshExpirer: refreshExpirer, accessTTL: accessTTL,
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
	now := time.Now().UTC()
	subject.Attributes = cloneStringMap(subject.Attributes)
	if subject.Attributes == nil {
		subject.Attributes = map[string]string{}
	}
	if !subject.AuthenticatedAt.IsZero() {
		authTime := subject.AuthenticatedAt.UTC().Format(time.RFC3339)
		subject.Attributes["auth_time"] = authTime
	}

	// 颁发访问令牌
	accessToken, err := s.tokenCodec.IssueAccessToken(ctx, &AccessTokenSubject{
		UserID: subject.UserID, LoginIdentityID: subject.LoginIdentityID, SessionID: subject.SessionID,
		TenantID: subject.TenantID, TenantDomain: subject.TenantDomain, OrgID: subject.OrgID,
		AMR: append([]string(nil), subject.AMR...), AuthenticatedAt: subject.AuthenticatedAt,
		Attributes: cloneStringMap(subject.Attributes),
	}, s.accessTTL)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate access token")
	}

	// 颁发刷新令牌
	refreshToken, err := s.issueRefreshToken(subject, sess, now)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate refresh token")
	}
	return NewUserTokenSet(accessToken, refreshToken), nil
}

// issueRefreshToken 颁发刷新令牌。
func (s *tokenSetMinter) issueRefreshToken(subject *AccessTokenSubject, sess *sessiondomain.Session, now time.Time) (*RefreshToken, error) {
	refreshExpiresAt, err := s.refreshExpirer.NextRefreshExpiresAt(now, sess)
	if err != nil {
		return nil, err
	}
	// Session 持有认证上下文权威事实；新 RefreshToken 不再写入重复上下文。
	token := NewRefreshTokenWithExpiry(
		uuid.NewString(), uuid.NewString(), sess.SessionID, subject.UserID, subject.LoginIdentityID,
		subject.TenantID, nil, nil, refreshExpiresAt,
	)
	token.IssuedAt = now
	return token, nil
}

// serviceTokenIssuer 是服务令牌颁发器的实现。
type serviceTokenIssuer struct {
	tokenCodec BearerTokenCodec
	accessTTL  time.Duration
}

// newServiceTokenIssuer 创建服务令牌颁发器。
func newServiceTokenIssuer(tokenCodec BearerTokenCodec, accessTTL time.Duration) ServiceTokenIssuer {
	return &serviceTokenIssuer{tokenCodec: tokenCodec, accessTTL: accessTTL}
}

// IssueServiceToken 颁发服务令牌。
func (s *serviceTokenIssuer) IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*ServiceToken, error) {
	// 验证主题
	if strings.TrimSpace(subject) == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "subject is required")
	}
	if !hasNonEmptyAudience(audience) {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "audience is required")
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

func hasNonEmptyAudience(audience []string) bool {
	for _, value := range audience {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}
