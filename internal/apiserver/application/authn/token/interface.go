package token

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
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
