package session

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin"
	tokenapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// ApplicationService 是 transport 层依赖的用户会话门面（登录 / 续期 / 登出）。
type ApplicationService interface {
	Login(ctx context.Context, cmd LoginCommand) (*LoginResult, error)
	RenewSession(ctx context.Context, refreshToken string) (*RenewResult, error)
	Logout(ctx context.Context, cmd LogoutCommand) error
}

// Dependencies 是 ApplicationService 的装配依赖。
// SignIn 由 assembler 构建并注入，避免门面重复暴露 Authenticator / ProofFactory 等登录专用依赖。
type Dependencies struct {
	Refresher tokenapp.Refresher
	Revoker   tokenapp.Revoker
	SignIn    *signin.SignIn
}

type service struct {
	signIn  *signin.SignIn
	renewal *Renewal
	signOut *SignOut
}

var _ ApplicationService = (*service)(nil)

// NewApplicationService 创建用户会话应用服务。
func NewApplicationService(deps Dependencies) (ApplicationService, error) {
	if deps.Refresher == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "token refresher is required")
	}
	if deps.Revoker == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "token revoker is required")
	}
	if deps.SignIn == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "sign-in use case is required")
	}

	return &service{
		signIn:  deps.SignIn,
		renewal: &Renewal{refresher: deps.Refresher},
		signOut: &SignOut{revoker: deps.Revoker},
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
