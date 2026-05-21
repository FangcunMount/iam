package token

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// issuer 用于聚合 token 签发能力，仅供 token 包内部装配。
type issuer struct {
	sessionIssuer *sessionTokenIssuer // 用户会话令牌签发器
	serviceIssuer *serviceTokenIssuer // 服务令牌签发器
}

// 确保 issuer 实现 issuerPort 接口。
var _ issuerPort = (*issuer)(nil)

// newIssuer 创建 token 签发器。
func newIssuer(
	tokenCodec AccessTokenCodec,
	tokenStore Store,
	sessionCreator SessionCreator,
	refreshExpirer SessionRefreshExpirer,
	claimMapper ClaimMapper,
	accessTTL time.Duration,
) *issuer {
	claimMapper = normalizeClaimMapper(claimMapper)
	pairIssuer := newSessionTokenPairIssuer(tokenCodec, tokenStore, refreshExpirer, claimMapper, accessTTL)
	return &issuer{
		sessionIssuer: newSessionTokenIssuer(sessionCreator, pairIssuer),
		serviceIssuer: newServiceTokenIssuer(tokenCodec, accessTTL),
	}
}

// sessionTokenPairIssuer 返回基于既有 session 签发 token pair 的内部协作者。
func (s *issuer) sessionTokenPairIssuer() sessionTokenPairIssuerPort {
	if s == nil || s.sessionIssuer == nil {
		return nil
	}
	return s.sessionIssuer.pairIssuer
}

// IssueToken 颁发用户会话令牌。
func (s *issuer) IssueToken(ctx context.Context, principal *authentication.Principal) (*TokenPair, error) {
	return s.sessionIssuer.IssueToken(ctx, principal)
}

// IssueServiceToken 颁发服务令牌。
func (s *issuer) IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*TokenPair, error) {
	return s.serviceIssuer.IssueServiceToken(ctx, subject, audience, attributes, ttl)
}

// sessionTokenIssuer 用于签发用户会话令牌。
type sessionTokenIssuer struct {
	sessionCreator SessionCreator             // 会话创建器
	pairIssuer     sessionTokenPairIssuerPort // 令牌对签发器
}

// 确保 sessionTokenIssuer 实现 sessionTokenIssuerPort 接口。
var _ sessionTokenIssuerPort = (*sessionTokenIssuer)(nil)

// newSessionTokenIssuer 创建 sessionTokenIssuer。
func newSessionTokenIssuer(sessionCreator SessionCreator, pairIssuer sessionTokenPairIssuerPort) *sessionTokenIssuer {
	return &sessionTokenIssuer{
		sessionCreator: sessionCreator,
		pairIssuer:     pairIssuer,
	}
}

// IssueToken 颁发用户会话令牌。
func (s *sessionTokenIssuer) IssueToken(ctx context.Context, principal *authentication.Principal) (*TokenPair, error) {
	if principal == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "principal is required")
	}

	// 创建 session
	sess, err := s.sessionCreator.Create(ctx, principal)
	if err != nil {
		if perrors.IsCode(err, code.ErrInvalidArgument) {
			return nil, err
		}
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to create session")
	}

	// 颁发令牌对
	return s.pairIssuer.IssueTokenPair(ctx, principal, sess)
}

// serviceTokenIssuer 用于签发服务令牌。
type serviceTokenIssuer struct {
	tokenCodec AccessTokenCodec // 令牌编码器
	accessTTL  time.Duration    // 令牌有效期
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

// IssueServiceToken 颁发服务令牌。
func (s *serviceTokenIssuer) IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*TokenPair, error) {
	// 如果主题为空，则返回错误
	if subject == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "subject is required")
	}
	// 如果有效期小于等于0，则使用默认有效期
	if ttl <= 0 {
		ttl = s.accessTTL
	}
	// 颁发服务令牌
	serviceToken, err := s.tokenCodec.IssueServiceToken(ctx, subject, audience, attributes, ttl)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrInternalServerError, "failed to generate service token")
	}
	return NewTokenPair(serviceToken, nil), nil
}
