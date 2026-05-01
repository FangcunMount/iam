package token

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// SessionTokenIssuer 编排登录成功后的 session 创建、access token 签发和 refresh token 保存。
type SessionTokenIssuer interface {
	IssueToken(ctx context.Context, principal *authentication.Principal) (*TokenPair, error)
}

// ServiceTokenIssuer 签发服务间访问令牌；服务令牌不创建 refresh token。
type ServiceTokenIssuer interface {
	IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*TokenPair, error)
}

// AccessRevoker 撤销单个 access token 及其关联会话。
type AccessRevoker interface {
	RevokeAccessToken(ctx context.Context, tokenValue string) error
}

// Issuer 聚合 token 签发/撤销能力，保留给 login/container 现有装配使用。
type Issuer interface {
	SessionTokenIssuer
	ServiceTokenIssuer
	AccessRevoker
}

type Refresher interface {
	RefreshToken(ctx context.Context, refreshTokenValue string) (*TokenPair, error)
	RevokeRefreshToken(ctx context.Context, refreshTokenValue string) error
}

type Verifier interface {
	VerifyAccessToken(ctx context.Context, tokenValue string) (*TokenClaims, error)
}

// Store 保存 refresh token 与 access token 撤销标记。
type Store interface {
	SaveRefreshToken(ctx context.Context, token *Token) error
	GetRefreshToken(ctx context.Context, tokenValue string) (*Token, error)
	DeleteRefreshToken(ctx context.Context, tokenValue string) error
	MarkAccessTokenRevoked(ctx context.Context, tokenID string, expiry time.Duration) error
	IsAccessTokenRevoked(ctx context.Context, tokenID string) (bool, error)
}

// AccessTokenCodec 是 access/service token 编码适配端口；JWT 只是其中一种实现。
type AccessTokenCodec interface {
	IssueAccessToken(ctx context.Context, principal *Principal, expiresIn time.Duration) (*Token, error)
	IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, expiresIn time.Duration) (*Token, error)
	VerifyAccessToken(ctx context.Context, tokenValue string) (*TokenClaims, error)
}

// Principal 是 access token 编码所需的应用层身份快照。
type Principal struct {
	UserID    meta.ID
	AccountID meta.ID
	TenantID  meta.ID
	SessionID string
	AMR       []string
	Claims    map[string]any
}

// ClaimMapper 将认证主体附加信息转换为 refresh token 可持久化的字符串快照。
type ClaimMapper interface {
	Encode(map[string]any) map[string]string
	Decode(map[string]string) map[string]any
}

type stringClaimMapper struct{}

func NewStringClaimMapper() ClaimMapper {
	return stringClaimMapper{}
}

func (stringClaimMapper) Encode(in map[string]any) map[string]string {
	return stringifyClaims(in)
}

func (stringClaimMapper) Decode(in map[string]string) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func normalizeClaimMapper(mapper ClaimMapper) ClaimMapper {
	if mapper == nil {
		return stringClaimMapper{}
	}
	return mapper
}

type SessionManager = sessiondomain.Manager
type SubjectAccessEvaluator = sessiondomain.SubjectAccessEvaluator
