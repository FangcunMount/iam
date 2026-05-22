package session

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	credentialapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// ApplicationService 是 transport 层依赖的用户会话门面（登录 / 续期 / 登出）。
type ApplicationService interface {
	Login(ctx context.Context, cmd LoginCommand) (*LoginResult, error)
	RenewSession(ctx context.Context, refreshToken string) (*RenewResult, error)
	Logout(ctx context.Context, cmd LogoutCommand) error
}

// Dependencies 是 ApplicationService 的装配依赖。
type Dependencies struct {
	TokenService       tokenapp.TokenApplicationService
	Authenticator      *authentication.Authenticator
	MethodRegistry     MethodRegistry
	ProofFactory       ProofFactory
	CredentialRecorder credentialapp.Recorder
}

type service struct {
	signIn  *signin.SignIn
	renewal *Renewal
	signOut *SignOut
}

var _ ApplicationService = (*service)(nil)

// NewApplicationService 创建用户会话应用服务。
func NewApplicationService(deps Dependencies) (ApplicationService, error) {
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

	return &service{
		signIn: signin.New(signin.Dependencies{
			TokenService:       deps.TokenService,
			MethodRegistry:     deps.MethodRegistry,
			ProofFactory:       deps.ProofFactory,
			Authenticator:      deps.Authenticator,
			CredentialRecorder: deps.CredentialRecorder,
		}),
		renewal: &Renewal{tokenService: deps.TokenService},
		signOut: &SignOut{tokenService: deps.TokenService},
	}, nil
}

func (s *service) Login(ctx context.Context, cmd LoginCommand) (*LoginResult, error) {
	if s == nil || s.signIn == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "session service is not initialized")
	}
	return s.signIn.Execute(ctx, cmd)
}

func (s *service) RenewSession(ctx context.Context, refreshToken string) (*RenewResult, error) {
	if s == nil || s.renewal == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "session service is not initialized")
	}
	return s.renewal.Execute(ctx, refreshToken)
}

func (s *service) Logout(ctx context.Context, cmd LogoutCommand) error {
	if s == nil || s.signOut == nil {
		return perrors.WithCode(code.ErrInvalidArgument, "session service is not initialized")
	}
	return s.signOut.Execute(ctx, cmd)
}
