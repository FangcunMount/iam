package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	credentialapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// LoginApplicationService 是 transport 层依赖的登录会话门面。
type LoginApplicationService interface {
	// Login 登录
	// 参数：ctx 上下文, cmd 登录命令
	// 返回：登录结果, 错误
	// 职责：执行登录逻辑，返回登录结果
	Login(ctx context.Context, cmd LoginCommand) (*LoginResult, error)

	// Reauthenticate 再验证
	// 参数：ctx 上下文, token 访问令牌
	// 返回：认证结果, 错误
	// 职责：再验证访问令牌是否仍然有效，返回认证结果
	Reauthenticate(ctx context.Context, token string) (*AuthResult, error)

	// Logout 登出
	// 参数：ctx 上下文, cmd 登出命令
	// 返回：错误
	// 职责：执行登出逻辑，返回错误
	Logout(ctx context.Context, cmd LogoutCommand) error
}

// Dependencies 是 LoginApplicationService 的装配依赖。
type Dependencies struct {
	TokenService       tokenapp.TokenApplicationService
	Authenticator      *authentication.Authenticator
	MethodRegistry     MethodRegistry
	ProofFactory       ProofFactory
	ReAuthenticator    ReAuthenticator
	CredentialRecorder credentialapp.Recorder
}

type service struct {
	signIn         *signin.SignIn  // 登录编排
	reauthenticate *Reauthenticate // 再验证编排
	signOut        *SignOut        // 登出编排
}

var _ LoginApplicationService = (*service)(nil)

// NewLoginApplicationService 创建登录应用服务。
// 参数：deps 依赖
// 返回：登录应用服务, 错误
// 职责：创建登录应用服务，返回登录应用服务
func NewLoginApplicationService(deps Dependencies) (LoginApplicationService, error) {
	// 确保依赖已准备好
	if deps.TokenService == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "token service is required")
	}
	if deps.Authenticator == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "authenticator is required")
	}
	if deps.MethodRegistry == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "method registry is required")
	}
	if deps.ProofFactory == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "proof factory is required")
	}
	if deps.ReAuthenticator == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "re-authenticator is required")
	}

	return &service{
		signIn: signin.New(signin.Dependencies{
			TokenService:       deps.TokenService,
			MethodRegistry:     deps.MethodRegistry,
			ProofFactory:       deps.ProofFactory,
			Authenticator:      deps.Authenticator,
			CredentialRecorder: deps.CredentialRecorder,
		}),
		reauthenticate: NewReauthenticate(deps.ReAuthenticator),
		signOut:        &SignOut{tokenService: deps.TokenService},
	}, nil
}

// Login 登录
// 参数：ctx 上下文, cmd 登录命令
// 返回：登录结果, 错误
// 职责：执行登录逻辑，返回登录结果
func (s *service) Login(ctx context.Context, cmd LoginCommand) (*LoginResult, error) {
	if s == nil || s.signIn == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "login service is not initialized")
	}
	return s.signIn.Execute(ctx, cmd)
}

// Reauthenticate 再验证
// 参数：ctx 上下文, token 访问令牌
// 返回：认证结果, 错误
// 职责：再验证访问令牌是否仍然有效，返回认证结果
func (s *service) Reauthenticate(ctx context.Context, token string) (*AuthResult, error) {
	if s == nil || s.reauthenticate == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "login service is not initialized")
	}

	// 再验证访问令牌是否仍然有效
	principal, err := s.reauthenticate.Execute(ctx, token)
	if err != nil {
		return nil, err
	}

	// 由认证主体构造认证结果
	return authResultFromPrincipal(principal), nil
}

// Logout 登出
// 参数：ctx 上下文, cmd 登出命令
// 返回：错误
// 职责：执行登出逻辑，返回错误
func (s *service) Logout(ctx context.Context, cmd LogoutCommand) error {
	if s == nil || s.signOut == nil {
		return perrors.WithCode(code.ErrInvalidArgument, "login service is not initialized")
	}
	return s.signOut.Execute(ctx, cmd)
}
