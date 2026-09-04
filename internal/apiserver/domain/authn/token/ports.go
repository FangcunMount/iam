package token

import (
	"context"
	"time"

	admissiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Store 持久化 RefreshToken、消费事实与 Bearer Token 撤销事实。
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
	// MarkBearerTokenRevoked 标记 access/service bearer token 已撤销
	MarkBearerTokenRevoked(ctx context.Context, tokenID string, expiry time.Duration) error
	// IsBearerTokenRevoked 检查 access/service bearer token 是否已撤销
	IsBearerTokenRevoked(ctx context.Context, tokenID string) (bool, error)
}

// BearerTokenCodec 对 access/service bearer token 进行编码和密码学验证。
// 领域只依赖该能力，不感知 JWT/JWS 等 wire format。
type BearerTokenCodec interface {
	// IssueAccessToken 颁发访问令牌
	IssueAccessToken(ctx context.Context, subject *AccessTokenSubject, expiresIn time.Duration) (*AccessToken, error)
	// IssueServiceToken 颁发服务令牌
	IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, expiresIn time.Duration) (*ServiceToken, error)
	// VerifyBearerToken 验证 access/service bearer token
	VerifyBearerToken(ctx context.Context, tokenValue string) (*VerifiedTokenClaims, error)
}

// AccessTokenSubject 访问令牌编码所需的已绑定 Session 的认证主体快照。
// 领域层完成投影后，JWT adapter 只负责序列化，不再从任意 Claims 推断授权域。
type AccessTokenSubject struct {
	UserID          meta.ID
	LoginIdentityID meta.ID
	TenantID        meta.ID
	SessionID       string
	TenantDomain    string
	OrgID           string
	AMR             []string
	AuthenticatedAt time.Time
	// Attributes 是已经过准入的对外附加字段，不再代表任意 Principal.Claims。
	Attributes map[string]string
}

// LegacyAuthenticationContextSnapshotDecoder 只负责读取迁移前 RefreshToken 中的认证上下文快照。
// 新写入以 Session.AuthContext/TokenContext 为权威来源，不再生成该快照。
type LegacyAuthenticationContextSnapshotDecoder interface {
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
	VerifyToken(ctx context.Context, tokenValue string) (*VerifiedTokenClaims, error)
}

// Revoker 撤销 bearer token 及其关联 Session。
type Revoker interface {
	// RevokeBearerToken 撤销令牌。
	RevokeBearerToken(ctx context.Context, tokenValue string) error
}
