package token

import (
	"context"
	"time"

	admissiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Store 持久化 RefreshToken、消费事实与 AccessToken 撤销事实。
type Store interface {
	// SaveRefreshToken 保存刷新令牌
	SaveRefreshToken(ctx context.Context, token *RefreshToken) error
	// GetRefreshToken 获取刷新令牌
	GetRefreshToken(ctx context.Context, tokenValue string) (*RefreshToken, error)
	// GetConsumedRefreshToken 获取已消费的刷新令牌
	GetConsumedRefreshToken(ctx context.Context, tokenValue string) (*ConsumedRefreshToken, error)
	// RotateRefreshToken 轮换刷新令牌
	RotateRefreshToken(ctx context.Context, oldValue, expectedOldID string, newToken *RefreshToken) (bool, error)
	// DeleteRefreshToken 删除刷新令牌
	DeleteRefreshToken(ctx context.Context, tokenValue string) error
	// MarkAccessTokenRevoked 标记访问令牌已撤销
	MarkAccessTokenRevoked(ctx context.Context, tokenID string, expiry time.Duration) error
	// IsAccessTokenRevoked 检查访问令牌是否已撤销
	IsAccessTokenRevoked(ctx context.Context, tokenID string) (bool, error)
}

// AccessTokenCodec 访问令牌编码器
// 它负责将访问令牌和服务令牌编码为 JWT，并验证 JWT 的签名
// 当前的实现是 JWT，未来可以支持其他编码方式
type AccessTokenCodec interface {
	// IssueAccessToken 颁发访问令牌
	IssueAccessToken(ctx context.Context, subject *AccessTokenSubject, expiresIn time.Duration) (*AccessToken, error)
	// IssueServiceToken 颁发服务令牌
	IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, expiresIn time.Duration) (*ServiceToken, error)
	// VerifyAccessToken 验证访问令牌
	VerifyAccessToken(ctx context.Context, tokenValue string) (*TokenClaims, error)
}

// AccessTokenSubject 访问令牌编码所需的已绑定 Session 的认证主体快照
// 它包含了访问令牌的认证主体信息，包括用户 ID、登录身份 ID、租户 ID、会话 ID、认证方法、认证域和认证方法引用
type AccessTokenSubject struct {
	UserID          meta.ID
	LoginIdentityID meta.ID
	TenantID        meta.ID
	SessionID       string
	AuthMethod      string
	Realm           string
	AMR             []string
	Claims          map[string]any
}

// RefreshClaimsCodec 将认证声明编码为 Session/RefreshToken 共用的快照
// 它负责将认证声明编码为 Session/RefreshToken 共用的快照，包括用户 ID、登录身份 ID、租户 ID、会话 ID、认证方法、认证域和认证方法引用
type RefreshClaimsCodec interface {
	Encode(map[string]any) map[string]string
	Decode(map[string]string) map[string]any
}

// SessionLoader 是会话加载器
type SessionLoader = sessiondomain.Loader

// SessionRevoker 是会话撤销器
type SessionRevoker = sessiondomain.Revoker

// SessionExtender 是会话延期器
type SessionExtender = sessiondomain.Extender

// SessionRefreshExpirer 是会话刷新过期时间计算器
type SessionRefreshExpirer = sessiondomain.RefreshExpirer

// AdmissionPolicy 是认证准入策略
type AdmissionPolicy = admissiondomain.Policy

// TokenSetMinter 在既有 Session 上签发尚未持久化的用户令牌集合
type TokenSetMinter interface {
	// MintTokenSet 颁发用户令牌。
	MintTokenSet(ctx context.Context, principal *authentication.Principal, sess *sessiondomain.Session) (*UserTokenSet, error)
}

// ServiceTokenIssuer 签发不绑定用户 Session 的服务令牌。
type ServiceTokenIssuer interface {
	// IssueServiceToken 颁发服务令牌。
	IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, ttl time.Duration) (*ServiceToken, error)
}

// Refresher 轮换 RefreshToken 并延续认证状态。
type Refresher interface {
	// RefreshToken 轮换刷新令牌。
	RefreshToken(ctx context.Context, refreshTokenValue string) (*UserTokenSet, error)
	// RevokeRefreshToken 撤销刷新令牌。
	RevokeRefreshToken(ctx context.Context, refreshTokenValue string) error
}

// Verifier 在线验证 access/service token 及用户认证状态。
type Verifier interface {
	// VerifyToken 验证令牌。
	VerifyToken(ctx context.Context, tokenValue string) (*TokenClaims, error)
}

// Revoker 撤销 bearer token 及其关联 Session。
type Revoker interface {
	// RevokeBearerToken 撤销令牌。
	RevokeBearerToken(ctx context.Context, tokenValue string) error
}
