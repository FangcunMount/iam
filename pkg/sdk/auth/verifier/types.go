package verifier

import (
	"context"
	"time"

	authnv1 "github.com/FangcunMount/iam/api/grpc/iam/authn/v1"
	"github.com/FangcunMount/iam/pkg/sdk/config"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// VerifyTokenClient 定义远程验证所需的最小客户端能力。
type VerifyTokenClient interface {
	VerifyToken(context.Context, *authnv1.VerifyTokenRequest) (*authnv1.VerifyTokenResponse, error)
}

// VerifyStrategy 定义 Token 验证策略接口。
type VerifyStrategy interface {
	Verify(ctx context.Context, token string, opts *VerifyOptions) (*VerifyResult, error)
	Name() string
}

// VerifyResult 验证结果。
type VerifyResult struct {
	Valid    bool
	Claims   *TokenClaims
	Metadata *VerifyMetadata
	RawToken jwt.Token
}

// VerifyMetadata Token 验证元数据。
type VerifyMetadata struct {
	TokenType authnv1.TokenType
	Status    authnv1.TokenStatus
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// TokenClaims Token 声明。
type TokenClaims struct {
	TokenID   string
	Subject   string
	SessionID string
	Issuer    string
	Audience  []string
	ExpiresAt time.Time
	IssuedAt  time.Time
	NotBefore time.Time
	UserID    string
	AccountID string
	TenantID  string
	Roles     []string
	Scopes    []string
	TokenType string
	AMR       []string
	Extra     map[string]interface{}
}

// VerifyOptions 验证选项。
type VerifyOptions struct {
	ForceRemote      bool
	IncludeMetadata  bool
	ExpectedAudience []string
	ExpectedIssuer   string
}

// VerifyResultCache 验证结果缓存接口。
type VerifyResultCache interface {
	Get(token string) (*VerifyResult, bool)
	Set(token string, result *VerifyResult, ttl time.Duration)
}

// TokenVerifier Token 验证器（使用策略模式）。
type TokenVerifier struct {
	config         *config.TokenVerifyConfig
	strategy       VerifyStrategy
	remoteStrategy VerifyStrategy
}

// TokenVerifierOption 验证器配置选项。
type TokenVerifierOption func(*TokenVerifier)
