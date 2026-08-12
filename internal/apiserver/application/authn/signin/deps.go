package signin

import (
	"context"

	credentialapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/credential"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin/method"
	tokenapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
)

// MethodRegistry 选择登录方式并解析 payload。
type MethodRegistry interface {
	Select(context.Context, method.LoginRequest) (method.LoginMethodSelection, error)
}

// ProofFactory 将登录方式选择结果构造成领域认证凭据。
type ProofFactory interface {
	Build(context.Context, method.LoginMethodSelection) (authentication.AuthCredential, error)
}

// Dependencies 是 SignIn 用例依赖。
type Dependencies struct {
	TokenService       tokenapp.TokenApplicationService
	MethodRegistry     MethodRegistry
	ProofFactory       ProofFactory
	Authenticator      *authentication.Authenticator
	CredentialRecorder credentialapp.Recorder
	AccessChecker      sessiondomain.SubjectAccessEvaluator
}
