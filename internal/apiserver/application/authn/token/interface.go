package token

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
)

// ================== Interface Interfaces (Driving Ports) ==================

// TokenApplicationService 令牌应用服务
// 职责：管理用户会话令牌和服务间访问令牌的签发、刷新、撤销和验证。
type TokenApplicationService interface {
	// IssueToken 在登录完成后签发用户会话令牌。
	IssueToken(ctx context.Context, principal *authentication.Principal) (*TokenPair, error)

	// IssueServiceToken 签发服务间访问令牌。
	IssueServiceToken(ctx context.Context, req IssueServiceTokenRequest) (*TokenIssueResult, error)

	// RefreshToken 刷新访问令牌
	RefreshToken(ctx context.Context, refreshToken string) (*TokenRefreshResult, error)

	// RevokeAccessToken 撤销访问令牌
	RevokeAccessToken(ctx context.Context, accessToken string) error

	// RevokeRefreshToken 撤销刷新令牌
	RevokeRefreshToken(ctx context.Context, refreshToken string) error

	// VerifyToken 验证访问令牌
	VerifyToken(ctx context.Context, req VerifyTokenRequest) (*TokenVerifyResult, error)
}

// ClaimMapper 将认证主体附加信息转换为 refresh token 可持久化的字符串快照。
type ClaimMapper interface {
	// Encode 编码声明
	Encode(map[string]any) map[string]string
	// Decode 解码声明
	Decode(map[string]string) map[string]any
}

// sessionTokenPairIssuerPort 基于已存在的 session 签发 access token 并保存 refresh token。
//
// Login 会先创建 session 再调用该组件；Refresh 会复用已有 session 后调用该组件。
type sessionTokenPairIssuerPort interface {
	// IssueTokenPair 根据认证主体和会话信息签发 access token，并保存新的 refresh token。
	// 职责：根据认证主体和会话信息签发 access token，并保存新的 refresh token。
	// 返回值：访问令牌对
	IssueTokenPair(ctx context.Context, principal *authentication.Principal, sess *sessiondomain.Session) (*TokenPair, error)
}

// issuerPort 用于聚合 token 签发能力，仅供 token 包内部装配。
type issuerPort interface {
	sessionTokenIssuerPort
	serviceTokenIssuerPort
}

// sessionTokenIssuerPort 用于签发用户会话令牌。
type sessionTokenIssuerPort interface {
	// IssueToken 签发用户会话令牌
	// 职责：签发用户会话令牌。
	// 返回值：访问令牌对
	IssueToken(ctx context.Context, principal *authentication.Principal) (*TokenPair, error)
}

// serviceTokenIssuerPort 用于签发服务间访问令牌。
type serviceTokenIssuerPort interface {
	// IssueServiceToken 签发服务间访问令牌
	// 职责：签发服务间访问令牌。
	// 返回值：访问令牌对
	IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*TokenPair, error)
}

// revokerPort 撤销令牌和关联会话。
type revokerPort interface {
	// RevokeAccessToken 撤销访问令牌
	// 职责：撤销单个 access token 及其关联会话。
	RevokeAccessToken(ctx context.Context, tokenValue string) error
}

// refresherPort 根据 refresh token 刷新 access token 和 refresh token。
type refresherPort interface {
	// RefreshToken 刷新访问令牌
	// 职责：根据 refresh token 刷新 access token 和 refresh token。
	// 返回值：访问令牌对
	RefreshToken(ctx context.Context, refreshTokenValue string) (*TokenPair, error)

	// RevokeRefreshToken 删除刷新令牌
	// 职责：删除刷新令牌
	RevokeRefreshToken(ctx context.Context, refreshTokenValue string) error
}

// verifierPort 在线验证 access token，并检查可选的 issuer/audience 约束。
type verifierPort interface {
	// VerifyAccessToken 验证访问令牌
	// 职责：验证访问令牌是否有效：1. 令牌是否已撤销；2. 会话是否活跃；3. 主体访问权限是否允许。
	// 返回值：访问令牌声明
	VerifyAccessToken(ctx context.Context, tokenValue string) (*TokenClaims, error)
}

// ================== DTOs ==================

// IssueServiceTokenRequest 服务令牌签发请求。
type IssueServiceTokenRequest struct {
	Subject    string            // 令牌主体
	Audience   []string          // 受众
	TTL        time.Duration     // 令牌有效期
	Attributes map[string]string // 属性
}

// TokenIssueResult 令牌签发结果 DTO。
type TokenIssueResult struct {
	TokenPair *TokenPair // 令牌对
}

// TokenRefreshResult 令牌刷新结果 DTO。
type TokenRefreshResult struct {
	TokenPair *TokenPair // 令牌对
}

// VerifyTokenRequest 令牌验证请求 DTO。
type VerifyTokenRequest struct {
	AccessToken      string   // 访问令牌
	ExpectedIssuer   string   // 预期签发者
	ExpectedAudience []string // 预期受众
}

// TokenVerifyResult 令牌验证结果 DTO。
type TokenVerifyResult struct {
	Valid       bool         // 令牌是否有效
	Claims      *TokenClaims // 令牌声明
	FailureCode int
}
