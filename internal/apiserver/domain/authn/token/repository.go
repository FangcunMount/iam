package token

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
)

// ================== Repository Interface (Driven Port) ==================
// 定义领域模型所依赖的仓储接口，由基础设施层提供实现

// TokenStore 令牌存储端口
// 用于存储和管理刷新令牌，以及已撤销的访问令牌标记
type TokenStore interface {
	// SaveRefreshToken 保存刷新令牌
	SaveRefreshToken(ctx context.Context, token *Token) error

	// GetRefreshToken 获取刷新令牌
	//
	// 如果令牌不存在或已过期，返回 nil
	GetRefreshToken(ctx context.Context, tokenValue string) (*Token, error)

	// DeleteRefreshToken 删除刷新令牌（用于撤销或刷新后删除旧令牌）
	DeleteRefreshToken(ctx context.Context, tokenValue string) error

	// MarkAccessTokenRevoked 标记访问令牌已撤销
	//
	// 参数:
	//   - tokenID: 令牌唯一标识
	//   - expiry: 撤销标记有效期（通常设置为令牌剩余有效期）
	MarkAccessTokenRevoked(ctx context.Context, tokenID string, expiry time.Duration) error

	// IsAccessTokenRevoked 检查访问令牌是否已撤销
	IsAccessTokenRevoked(ctx context.Context, tokenID string) (bool, error)
}

// TokenGenerator 令牌生成器端口
//
// 用于生成和解析 JWT 访问令牌
type TokenGenerator interface {
	// GenerateAccessToken 生成访问令牌（JWT）
	//
	// 参数:
	//   - principal: 认证主体（包含用户ID、账户ID等）
	//   - expiresIn: 有效期
	//
	// 返回:
	//   - token: 令牌对象（包含 JWT 字符串）
	GenerateAccessToken(ctx context.Context, principal *authentication.Principal, expiresIn time.Duration) (*Token, error)

	// GenerateServiceToken 生成服务间访问令牌（JWT）。
	GenerateServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, expiresIn time.Duration) (*Token, error)

	// ParseAccessToken 解析访问令牌
	//
	// 参数:
	//   - tokenValue: JWT 字符串
	//
	// 返回:
	//   - claims: 令牌声明
	//   - err: 解析错误（如签名无效、过期等）
	ParseAccessToken(ctx context.Context, tokenValue string) (*TokenClaims, error)
}

// SessionStore 暴露 token 子域所需的会话生命周期能力。
type SessionStore = sessiondomain.Store

// SessionManager 暴露 token 子域所需的会话管理能力。
type SessionManager = sessiondomain.Manager

// SubjectAccessEvaluator 暴露 token 子域所需的访问主体状态判定能力。
type SubjectAccessEvaluator = sessiondomain.SubjectAccessEvaluator
