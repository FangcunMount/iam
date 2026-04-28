package token

import (
	"context"
	"time"

	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/token"
)

// ============= 应用服务接口（Driving Ports）=============

// TokenApplicationService 令牌应用服务 - 令牌管理
type TokenApplicationService interface {
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

// ============= DTOs =============

// IssueServiceTokenRequest 服务令牌签发请求。
type IssueServiceTokenRequest struct {
	Subject    string
	Audience   []string
	TTL        time.Duration
	Attributes map[string]string
}

// TokenIssueResult 令牌签发结果 DTO。
type TokenIssueResult struct {
	TokenPair *domain.TokenPair
}

// TokenRefreshResult 令牌刷新结果DTO
type TokenRefreshResult struct {
	TokenPair *domain.TokenPair // 新的令牌对
}

// VerifyTokenRequest 令牌验证请求 DTO。
type VerifyTokenRequest struct {
	AccessToken      string
	ExpectedIssuer   string
	ExpectedAudience []string
}

// TokenVerifyResult 令牌验证结果DTO
type TokenVerifyResult struct {
	Valid  bool                // 是否有效
	Claims *domain.TokenClaims // 令牌声明（如果有效）
}
