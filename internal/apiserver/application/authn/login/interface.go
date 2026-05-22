package login

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/method"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/proof"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
)

// MethodRegistry 选择登录方式并从命令中读取该方式对应 payload。
type MethodRegistry = method.Selector

// ProofFactory 将 method payload 构造成领域认证凭据。
type ProofFactory = proof.CredentialFactory

// SignInDependencies 是 SignIn 用例依赖（供装配层注入 signin）。
type SignInDependencies = signin.Dependencies

// ReAuthenticator 负责已有 access token 的再验证，不是登录行为。
type ReAuthenticator interface {
	// Reauthenticate 再验证 access token 是否仍然有效
	// 参数：ctx 上下文, tokenValue access token 值
	// 返回：认证决策, 错误
	// 职责：验证 access token 是否仍然有效，返回认证决策
	Reauthenticate(ctx context.Context, tokenValue string) (authentication.AuthDecision, error)
}

// TokenVerifier 是再验证需要的 token 能力。
// 职责：验证 access token 是否仍然有效，返回认证决策
type TokenVerifier interface {
	// VerifyToken 验证 access token 是否仍然有效
	// 参数：ctx 上下文, req 验证请求
	// 返回：验证结果, 错误
	// 职责：验证 access token 是否仍然有效，返回验证结果
	VerifyToken(ctx context.Context, req tokenapp.VerifyTokenRequest) (*tokenapp.TokenVerifyResult, error)
}
