package login

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// LoginApplicationService 是 transport 层依赖的登录应用服务门面。
//
// 它只表达 application 层对外提供的用例能力：
// 1. Login：完成认证并签发 TokenPair
// 2. Reauthenticate：验证已有 access token 是否仍然有效
// 3. Logout：撤销 AccessToken / RefreshToken
//
// 注意：
// - 它不负责解析 REST / gRPC wire payload；
// - 它不负责判断具体登录方式；
// - 它不负责构造领域 AuthCredential；
// - 这些职责分别下沉到 SignIn / MethodRegistry / ProofFactory。
type LoginApplicationService interface {
	Login(ctx context.Context, cmd LoginCommand) (*LoginResult, error)
	Reauthenticate(ctx context.Context, token string) (*AuthResult, error)
	Logout(ctx context.Context, cmd LogoutCommand) error
}

// Dependencies 是 LoginApplicationService 的装配依赖。
//
// service.go 只装配 use case，不直接装配具体登录方式。
// 具体 method 注册由外部完成后注入 MethodRegistry。
type Dependencies struct {
	TokenService tokenapp.TokenApplicationService

	Authenticator *authentication.Authenticator

	MethodRegistry  MethodRegistry
	ProofFactory    ProofFactory
	ReAuthenticator ReAuthenticator
}

// service 是 LoginApplicationService 的默认实现。
type service struct {
	signIn         *SignIn
	reauthenticate *Reauthenticate
	signOut        *SignOut
}

// 确保 service 实现了 LoginApplicationService 接口。
var _ LoginApplicationService = (*service)(nil)

// NewLoginApplicationService 创建登录应用服务。
func NewLoginApplicationService(deps Dependencies) (LoginApplicationService, error) {
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
		signIn: &SignIn{
			tokenService:        deps.TokenService,
			methodRegistry:      deps.MethodRegistry,
			proofFactory:        deps.ProofFactory,
			domainAuthenticator: deps.Authenticator,
		},
		reauthenticate: &Reauthenticate{
			reAuthenticator: deps.ReAuthenticator,
		},
		signOut: &SignOut{
			tokenService: deps.TokenService,
		},
	}, nil
}

// Login 登录。
func (s *service) Login(ctx context.Context, cmd LoginCommand) (*LoginResult, error) {
	if s == nil || s.signIn == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "login service is not initialized")
	}

	return s.signIn.Execute(ctx, cmd)
}

// Reauthenticate 登录态再验证。
func (s *service) Reauthenticate(ctx context.Context, token string) (*AuthResult, error) {
	if s == nil || s.reauthenticate == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "login service is not initialized")
	}

	return s.reauthenticate.Execute(ctx, token)
}

// Logout 登出。
func (s *service) Logout(ctx context.Context, cmd LogoutCommand) error {
	if s == nil || s.signOut == nil {
		return perrors.WithCode(code.ErrInvalidArgument, "login service is not initialized")
	}

	return s.signOut.Execute(ctx, cmd)
}
