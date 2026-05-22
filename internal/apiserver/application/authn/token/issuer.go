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

// issuerComponents 暴露装配所需的用户会话令牌与服务令牌协作者。
type issuerComponents struct {
	accessTokenIssuer  accessTokenIssuerPort
	serviceTokenIssuer serviceTokenIssuerPort
}

// newIssuer 创建令牌签发器
func newIssuer(
	tokenCodec AccessTokenCodec,
	tokenStore Store,
	sessionCreator SessionCreator,
	refreshExpirer SessionRefreshExpirer,
	refreshClaimsCodec RefreshClaimsCodec,
	accessTTL time.Duration,
) issuerComponents {
	return issuerComponents{
		accessTokenIssuer: newAccessTokenIssuer(
			sessionCreator,
			tokenCodec,
			tokenStore,
			refreshExpirer,
			refreshClaimsCodec,
			accessTTL,
		),
		serviceTokenIssuer: newServiceTokenIssuer(tokenCodec, accessTTL),
	}
}

// accessTokenIssuer 实现用户 access/refresh 令牌的登录签发与 refresh 轮换 mint。
type accessTokenIssuer struct {
	sessionCreator     SessionCreator
	tokenCodec         AccessTokenCodec
	tokenStore         Store
	refreshExpirer     SessionRefreshExpirer
	refreshClaimsCodec RefreshClaimsCodec
	accessTTL          time.Duration
}

// 确保 accessTokenIssuer 实现 accessTokenIssuerPort 接口。
var _ accessTokenIssuerPort = (*accessTokenIssuer)(nil)

// newAccessTokenIssuer 创建 accessTokenIssuer。
func newAccessTokenIssuer(
	sessionCreator SessionCreator,
	tokenCodec AccessTokenCodec,
	tokenStore Store,
	refreshExpirer SessionRefreshExpirer,
	refreshClaimsCodec RefreshClaimsCodec,
	accessTTL time.Duration,
) *accessTokenIssuer {
	return &accessTokenIssuer{
		sessionCreator:     sessionCreator,
		tokenCodec:         tokenCodec,
		tokenStore:         tokenStore,
		refreshExpirer:     refreshExpirer,
		refreshClaimsCodec: normalizeRefreshClaimsCodec(refreshClaimsCodec),
		accessTTL:          accessTTL,
	}
}

// IssueToken 登录：创建 session 并签发 access/refresh token pair。
func (s *accessTokenIssuer) IssueToken(ctx context.Context, principal *authentication.Principal) (*TokenPair, error) {
	if principal == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "principal is required")
	}

	sess, err := s.sessionCreator.Create(ctx, principal)
	if err != nil {
		if perrors.IsCode(err, code.ErrInvalidArgument) {
			return nil, err
		}
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to create session")
	}

	return s.MintTokenPair(ctx, principal, sess)
}

// MintTokenPair 在既有 session 上签发 access token 并保存新的 refresh token（Refresh 复用）。
func (s *accessTokenIssuer) MintTokenPair(ctx context.Context, principal *authentication.Principal, sess *sessiondomain.Session) (*TokenPair, error) {
	if principal == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "principal is required")
	}
	if sess == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "session is required")
	}
	if err := validatePrincipalSessionAlignment(principal, sess); err != nil {
		return nil, err
	}

	subject := accessTokenSubjectFromAuth(principal, sess)
	claims := cloneAnyMap(subject.Claims)

	now := time.Now().UTC()
	ensureAuthTime(claims, now)

	accessToken, err := s.tokenCodec.IssueAccessToken(ctx, &AccessTokenSubject{
		UserID:          subject.UserID,
		LoginIdentityID: subject.LoginIdentityID,
		SessionID:       subject.SessionID,
		TenantID:        subject.TenantID,
		AuthMethod:      subject.AuthMethod,
		Realm:           subject.Realm,
		AMR:             append([]string(nil), subject.AMR...),
		Claims:          claims,
	}, s.accessTTL)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate access token")
	}

	refreshToken, err := s.issueRefreshToken(ctx, subject, sess, claims, now)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate refresh token")
	}

	return NewTokenPair(accessToken, refreshToken), nil
}

// issueRefreshToken 在既有 session 上签发 refresh token。
func (s *accessTokenIssuer) issueRefreshToken(ctx context.Context, subject *AccessTokenSubject, sess *sessiondomain.Session, claims map[string]any, now time.Time) (*Token, error) {
	refreshExpiresAt, err := s.refreshExpirer.NextRefreshExpiresAt(now, sess)
	if err != nil {
		return nil, err
	}
	refreshToken := NewRefreshTokenWithExpiry(
		uuid.New().String(),
		uuid.New().String(),
		sess.SessionID,
		subject.UserID,
		subject.LoginIdentityID,
		subject.TenantID,
		subject.AMR,
		s.refreshClaimsCodec.Encode(claims),
		refreshExpiresAt,
	)

	refreshToken.IssuedAt = now
	refreshToken.AuthMethod = subject.AuthMethod
	refreshToken.Realm = subject.Realm

	if err := s.tokenStore.SaveRefreshToken(ctx, refreshToken); err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to save refresh token")
	}

	return refreshToken, nil
}

// ensureAuthTime 确保 auth_time 声明存在
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

// serviceTokenIssuer 用于签发服务令牌。
type serviceTokenIssuer struct {
	tokenCodec AccessTokenCodec
	accessTTL  time.Duration
}

// 确保 serviceTokenIssuer 实现 serviceTokenIssuerPort 接口。
var _ serviceTokenIssuerPort = (*serviceTokenIssuer)(nil)

// newServiceTokenIssuer 创建 serviceTokenIssuer。
func newServiceTokenIssuer(tokenCodec AccessTokenCodec, accessTTL time.Duration) *serviceTokenIssuer {
	return &serviceTokenIssuer{
		tokenCodec: tokenCodec,
		accessTTL:  accessTTL,
	}
}

// IssueServiceToken 签发服务间访问令牌。
func (s *serviceTokenIssuer) IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*TokenPair, error) {
	if subject == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "subject is required")
	}
	if ttl <= 0 {
		ttl = s.accessTTL
	}
	serviceToken, err := s.tokenCodec.IssueServiceToken(ctx, subject, audience, attributes, ttl)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate service token")
	}
	return NewTokenPair(serviceToken, nil), nil
}
