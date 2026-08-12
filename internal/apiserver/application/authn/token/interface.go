package token

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
)

// ==========================================================================
// ================== Interface Interfaces (Driving Ports) ==================
// ==========================================================================

// TokenApplicationService 令牌应用服务，对外门面接口
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

// RefreshClaimsCodec 将认证主体附加 claims 编码为 refresh/session 共用的字符串快照。
type RefreshClaimsCodec interface {
	Encode(map[string]any) map[string]string
	Decode(map[string]string) map[string]any
}

// accessTokenIssuerPort 用户 access/refresh 令牌签发：登录建 session 并签发，或在既有 session 上轮换 mint。
type accessTokenIssuerPort interface {
	// IssueToken 登录：创建 session 并签发 access/refresh token pair。
	IssueToken(ctx context.Context, principal *authentication.Principal) (*TokenPair, error)
	// MintTokenPair 在既有 session 上生成尚未持久化的 access/refresh token pair。
	MintTokenPair(ctx context.Context, principal *authentication.Principal, sess *sessiondomain.Session) (*TokenPair, error)
}

// serviceTokenIssuerPort 用于签发服务间访问令牌。
type serviceTokenIssuerPort interface {
	// IssueServiceToken 签发服务间访问令牌
	// 职责：签发服务间访问令牌
	// 返回值：访问令牌对
	IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*TokenPair, error)
}

// refresherPort 根据 refresh token 刷新 access token 和 refresh token。
type refresherPort interface {
	// RefreshToken 刷新访问令牌
	// 职责：根据 refresh token 刷新 access token 和 refresh token
	// 返回值：访问令牌对
	RefreshToken(ctx context.Context, refreshTokenValue string) (*TokenPair, error)

	// RevokeRefreshToken 删除刷新令牌
	// 职责：删除刷新令牌，并标记访问令牌已撤销
	RevokeRefreshToken(ctx context.Context, refreshTokenValue string) error
}

// verifierPort 在线验证 JWT bearer（用户 access 或 service access）。
type verifierPort interface {
	// VerifyToken 验证 bearer：解析 JWT；用户令牌另查撤销表、session 与主体状态。
	VerifyToken(ctx context.Context, tokenValue string) (*TokenClaims, error)
}

// revokerPort 撤销 JWT bearer 并视情况撤销关联会话。
type revokerPort interface {
	// RevokeBearerToken 撤销 JWT bearer（用户 access 或 service access）。
	RevokeBearerToken(ctx context.Context, tokenValue string) error
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
