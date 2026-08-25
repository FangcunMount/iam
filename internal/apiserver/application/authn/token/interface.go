package token

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
)

// SessionEstablisher 在认证成功后建立在线会话并返回会话令牌。
// 调用方无需感知 access token、refresh token 与 Session 的内部装配过程。
type SessionEstablisher interface {
	EstablishSession(ctx context.Context, principal *authentication.Principal) (*TokenPair, error)
}

// ServiceTokenIssuer 签发不绑定用户 Session 的服务间访问令牌。
type ServiceTokenIssuer interface {
	IssueServiceToken(ctx context.Context, req IssueServiceTokenRequest) (*TokenIssueResult, error)
}

// Refresher 通过 refresh token 轮换在线会话令牌。
type Refresher interface {
	RefreshToken(ctx context.Context, refreshToken string) (*TokenRefreshResult, error)
}

// Revoker 撤销访问令牌或 refresh token，并按令牌类型收敛关联会话状态。
type Revoker interface {
	RevokeAccessToken(ctx context.Context, accessToken string) error
	RevokeRefreshToken(ctx context.Context, refreshToken string) error
}

// Verifier 在线验证访问令牌及其可选 issuer / audience 约束。
type Verifier interface {
	VerifyToken(ctx context.Context, req VerifyTokenRequest) (*TokenVerifyResult, error)
}

// Capabilities 是组合根输出的令牌用例能力集合。
// 它只承载窄接口，不是供业务代码依赖的统一门面。
type Capabilities struct {
	SessionEstablisher SessionEstablisher
	ServiceTokenIssuer ServiceTokenIssuer
	Refresher          Refresher
	Revoker            Revoker
	Verifier           Verifier
}

// RefreshClaimsCodec 将认证主体附加 claims 编码为 refresh/session 共用的字符串快照。
type RefreshClaimsCodec interface {
	Encode(map[string]any) map[string]string
	Decode(map[string]string) map[string]any
}

// sessionEstablisherPort 创建 Session，并持久化首个 refresh token。
type sessionEstablisherPort interface {
	EstablishSession(ctx context.Context, principal *authentication.Principal) (*TokenPair, error)
}

// tokenPairMinterPort 在既有 Session 上生成尚未持久化的 access/refresh token pair。
type tokenPairMinterPort interface {
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
