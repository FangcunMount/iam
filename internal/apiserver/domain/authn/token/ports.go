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
	SaveRefreshToken(ctx context.Context, token *RefreshToken) error
	GetRefreshToken(ctx context.Context, tokenValue string) (*RefreshToken, error)
	GetConsumedRefreshToken(ctx context.Context, tokenValue string) (*ConsumedRefreshToken, error)
	RotateRefreshToken(ctx context.Context, oldValue, expectedOldID string, newToken *RefreshToken) (bool, error)
	DeleteRefreshToken(ctx context.Context, tokenValue string) error
	MarkAccessTokenRevoked(ctx context.Context, tokenID string, expiry time.Duration) error
	IsAccessTokenRevoked(ctx context.Context, tokenID string) (bool, error)
}

// AccessTokenCodec 是 access/service token 的编码和验签端口；JWT 只是其中一种实现。
type AccessTokenCodec interface {
	IssueAccessToken(ctx context.Context, subject *AccessTokenSubject, expiresIn time.Duration) (*AccessToken, error)
	IssueServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, expiresIn time.Duration) (*ServiceToken, error)
	VerifyAccessToken(ctx context.Context, tokenValue string) (*TokenClaims, error)
}

// AccessTokenSubject 是访问令牌编码所需的已绑定 Session 的认证主体快照。
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

// RefreshClaimsCodec 将认证声明编码为 Session/RefreshToken 共用的快照。
type RefreshClaimsCodec interface {
	Encode(map[string]any) map[string]string
	Decode(map[string]string) map[string]any
}

type SessionCreator = sessiondomain.Creator
type SessionLoader = sessiondomain.Loader
type SessionRevoker = sessiondomain.Revoker
type SessionExtender = sessiondomain.Extender
type SessionRefreshExpirer = sessiondomain.RefreshExpirer
type AdmissionPolicy = admissiondomain.Policy

// Issuer 将已认证主体颁发为完整在线认证结果（Session + TokenSet）。
type Issuer interface {
	Issue(ctx context.Context, principal *authentication.Principal) (*AuthenticationGrant, error)
}

// TokenSetMinter 在既有 Session 上签发尚未持久化的用户令牌集合。
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
