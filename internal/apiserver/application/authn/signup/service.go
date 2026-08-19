package signup

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/uow"
	idpresolver "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	userDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// signupService 实现 SignupService（见 interface.go）。
type signupService struct {
	uow                     uow.UnitOfWork
	prepareStep             *prepareStep
	resolveUserStep         *resolveUserStep
	ensureLoginIdentityStep *ensureLoginIdentityStep
	ensureCredentialStep    *ensureCredentialStep
}

var _ SignupService = (*signupService)(nil)

// NewSignupService 创建 Signup 应用服务。
func NewSignupService(
	uow uow.UnitOfWork,
	hasher authentication.PasswordHasher,
	externalIdentity idpresolver.Resolver,
	userRepo userDomain.Repository,
) SignupService {
	return &signupService{
		uow:                     uow,
		prepareStep:             newPrepareStep(externalIdentity),
		resolveUserStep:         newResolveUserStep(userRepo),
		ensureLoginIdentityStep: newEnsureLoginIdentityStep(),
		ensureCredentialStep:    newEnsureCredentialStep(hasher),
	}
}

func (s *signupService) SignUp(ctx context.Context, req SignupRequest) (*SignupResult, error) {
	prepared, err := s.prepareStep.Run(ctx, req)
	if err != nil {
		return nil, err
	}

	var out *signupExecutionResult
	err = s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		repos := registrationRepositories{
			Users:           tx.Users,
			Credentials:     tx.Credentials,
			LoginIdentities: tx.LoginIdentities,
		}

		userResult, err := s.resolveUserStep.Run(txCtx, repos, prepared)
		if err != nil {
			return err
		}

		loginIdentityResult, err := s.ensureLoginIdentityStep.Run(txCtx, repos.LoginIdentities, prepared, userResult.User.ID)
		if err != nil {
			return err
		}

		credentialResult, err := s.ensureCredentialStep.Run(txCtx, repos.Credentials, loginIdentityResult, prepared)
		if err != nil {
			return err
		}

		out = &signupExecutionResult{
			User:          userResult,
			LoginIdentity: loginIdentityResult,
			Credential:    credentialResult,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return buildSignupResult(out), nil
}

func buildSignupResult(out *signupExecutionResult) *SignupResult {
	user := out.User.User
	loginIdentity := out.LoginIdentity.Identity
	var credential *SignupCredential
	if out.Credential != nil && out.Credential.Credential != nil {
		credential = &SignupCredential{
			ID:   out.Credential.Credential.ID,
			Type: out.Credential.Credential.Type,
		}
	}

	return &SignupResult{
		UserID:             user.ID,
		UserName:           user.Name,
		Phone:              user.Phone,
		Email:              user.Email,
		UserStatus:         user.Status,
		LoginIdentityID:    loginIdentity.ID,
		Credential:         credential,
		IsNewUser:          out.User.IsNewUser(),
		IsNewLoginIdentity: out.LoginIdentity.IsNewLoginIdentity(),
	}
}

func isRepositoryNotFound(err error) bool {
	return perrors.IsCode(err, code.ErrUserNotFound) ||
		perrors.IsCode(err, code.ErrCredentialNotFound)
}
