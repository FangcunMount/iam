package session

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/method"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/proof"
)

// MethodRegistry 选择登录方式并从命令中读取该方式对应 payload。
type MethodRegistry = method.Selector

// ProofFactory 将 method payload 构造成领域认证凭据。
type ProofFactory = proof.CredentialFactory

// SignInDependencies 是 SignIn 用例依赖（供装配层注入 signin）。
type SignInDependencies = signin.Dependencies

// Revoker 提供管理员会话撤销能力。
type Revoker interface {
	RevokeSession(ctx context.Context, sessionID string, reason string, revokedBy string) error
	RevokeAllSessionsByLoginIdentity(ctx context.Context, loginIdentityID string, reason string, revokedBy string) error
	RevokeAllSessionsByUser(ctx context.Context, userID string, reason string, revokedBy string) error
}
