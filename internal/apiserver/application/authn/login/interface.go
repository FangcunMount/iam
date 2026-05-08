package login

import (
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/method"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/proof"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/reauth"
)

// MethodRegistry 选择登录方式并从命令中读取该方式对应 payload。
type MethodRegistry = method.Selector

// ProofFactory 将 method payload 构造成领域认证凭据。
type ProofFactory = proof.CredentialFactory

// ReAuthenticator 负责已有 access token 的再验证，不是登录行为。
type ReAuthenticator = reauth.ReAuthenticator
