package token

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ==================================================
// ================== Driven Ports ==================
// ==================================================

// Store 保存 refresh token 与 access token 撤销标记。
type Store interface {
	// SaveRefreshToken 保存刷新令牌
	SaveRefreshToken(ctx context.Context, token *Token) error
	// GetRefreshToken 获取刷新令牌
	GetRefreshToken(ctx context.Context, tokenValue string) (*Token, error)
	// DeleteRefreshToken 删除刷新令牌
	DeleteRefreshToken(ctx context.Context, tokenValue string) error
	// MarkAccessTokenRevoked 标记访问令牌已撤销
	MarkAccessTokenRevoked(ctx context.Context, tokenID string, expiry time.Duration) error
	// IsAccessTokenRevoked 检查访问令牌是否已撤销
	IsAccessTokenRevoked(ctx context.Context, tokenID string) (bool, error)
}

// AccessTokenCodec 是 access/service token 编码适配端口；JWT 只是其中一种实现。
type AccessTokenCodec interface {
	// IssueAccessToken 颁发访问令牌
	IssueAccessToken(ctx context.Context, subject *AccessTokenSubject, expiresIn time.Duration) (*Token, error)
	// IssueServiceToken 颁发服务令牌
	IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, expiresIn time.Duration) (*Token, error)
	// VerifyAccessToken 验证访问令牌
	VerifyAccessToken(ctx context.Context, tokenValue string) (*TokenClaims, error)
}

// AccessTokenSubject 是 access token 编码所需的应用层身份快照（已绑定 session）。
// JWT 授权域见 Claims["tenant_domain"] 或 Realm；TenantID 为 IAM 数值租户上下文。
type AccessTokenSubject struct {
	UserID          meta.ID        // 用户ID
	LoginIdentityID meta.ID        // 登录身份ID
	TenantID        meta.ID        // 租户ID
	SessionID       string         // 会话ID
	AuthMethod      string         // 认证方法
	Realm           string         // 认证域
	AMR             []string       // 认证方法引用
	Claims          map[string]any // 声明
}