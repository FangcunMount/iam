package token

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
)

// ====================================================
// ================== Driving Ports ===================
// ====================================================

// SessionTokenIssuer 是登录成功后对上层提供的用户会话令牌签发用例。
//
// 实现会创建 session、签发 access token，并保存 refresh token。
type SessionTokenIssuer interface {
	IssueToken(ctx context.Context, principal *authentication.Principal) (*TokenPair, error)
}

// ServiceTokenIssuer 签发服务间访问令牌；服务令牌不创建 session 或 refresh token。
type ServiceTokenIssuer interface {
	IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*TokenPair, error)
}

// AccessRevoker 撤销单个 access token 及其关联会话。
type AccessRevoker interface {
	RevokeAccessToken(ctx context.Context, tokenValue string) error
}

// RefreshRevoker 撤销单个 refresh token。
type RefreshRevoker interface {
	RevokeRefreshToken(ctx context.Context, tokenValue string) error
}

// Revoker 聚合 access token 与 refresh token 撤销能力。
type Revoker interface {
	AccessRevoker
	RefreshRevoker
}

// Issuer 聚合 token 签发/撤销能力，保留给 login/container 现有装配使用。
type Issuer interface {
	SessionTokenIssuer
	ServiceTokenIssuer
	AccessRevoker
}

// Refresher 根据 refresh token 刷新 access token 和 refresh token。
type Refresher interface {
	RefreshToken(ctx context.Context, refreshTokenValue string) (*TokenPair, error)
	RefreshRevoker
}

// Verifier 在线验证 access token。
type Verifier interface {
	VerifyAccessToken(ctx context.Context, tokenValue string) (*TokenClaims, error)
}
