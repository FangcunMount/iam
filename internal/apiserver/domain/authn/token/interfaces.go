package token

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
)

// ================== Domain Service Interfaces (Driving Ports) ==================
// 这些接口由领域层（领域服务）实现，供应用层调用
// 按照功能职责拆分，遵循接口隔离原则

// Issuer 令牌签发端口
type Issuer interface {
	// IssueToken 签发访问令牌和刷新令牌
	//
	// 参数:
	//   - principal: 认证主体（包含用户ID、账户ID等信息）
	//
	// 返回:
	//   - TokenPair: 访问令牌和刷新令牌对
	//   - err: 错误信息
	IssueToken(ctx context.Context, principal *authentication.Principal) (*TokenPair, error)

	// IssueServiceToken 签发服务间访问令牌。
	IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*TokenPair, error)

	// RevokeAccessToken 撤销访问令牌，并联动当前会话。
	RevokeAccessToken(ctx context.Context, tokenValue string) error
}

// Refresher 令牌刷新端口
type Refresher interface {
	// RefreshToken 刷新访问令牌
	//
	// 参数:
	//   - refreshTokenValue: 刷新令牌字符串
	//
	// 返回:
	//   - TokenPair: 新的访问令牌和刷新令牌对
	//   - err: 错误信息
	RefreshToken(ctx context.Context, refreshTokenValue string) (*TokenPair, error)

	// RevokeRefreshToken 撤销刷新令牌
	RevokeRefreshToken(ctx context.Context, refreshTokenValue string) error
}

// Verifier 令牌验证端口
type Verifier interface {
	// VerifyAccessToken 验证访问令牌
	//
	// 参数:
	//   - tokenValue: 访问令牌字符串
	//
	// 返回:
	//   - TokenClaims: 令牌声明信息
	//   - err: 错误信息
	VerifyAccessToken(ctx context.Context, tokenValue string) (*TokenClaims, error)
}
