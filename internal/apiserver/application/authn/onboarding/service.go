package onboarding

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/uow"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	idpPort "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// loginIdentityOnboarder 登录身份开通器
type loginIdentityOnboarder struct {
	uow                  uow.UnitOfWork
	preparer             *requestPreparer
	userResolver         *userResolver
	loginIdentityEnsurer *loginIdentityEnsurer
	credentialEnsurer    *credentialEnsurer
}

// 确保登录身份开通服务实现 LoginIdentityOnboarder 接口。
var _ LoginIdentityOnboarder = (*loginIdentityOnboarder)(nil)

// NewLoginIdentityOnboarder 创建登录身份开通服务。
func NewLoginIdentityOnboarder(
	uow uow.UnitOfWork,
	hasher authentication.PasswordHasher,
	idp authentication.IdentityProvider,
	userRepo userDomain.Repository,
	wechatAppQuerier idpPort.Repository,
	secretVault idpPort.SecretVault,
) LoginIdentityOnboarder {
	wechatIdentityResolver := newWechatIdentityResolver(idp, wechatAppQuerier, secretVault)
	return &loginIdentityOnboarder{
		uow:                  uow,
		preparer:             newRequestPreparer(wechatIdentityResolver),
		userResolver:         newUserResolver(userRepo),
		loginIdentityEnsurer: newLoginIdentityEnsurer(),
		credentialEnsurer:    newCredentialEnsurer(hasher),
	}
}

// Onboard 统一登录身份开通接口。
func (s *loginIdentityOnboarder) Onboard(ctx context.Context, req OnboardingRequest) (*OnboardingResult, error) {
	// 准备登录身份数据。
	prepared, err := s.preparer.Prepare(ctx, req)
	if err != nil {
		return nil, err
	}

	// 在事务中执行登录身份开通流程。
	var out *onboardingExecutionResult
	err = s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		repos := registrationRepositories{
			Users:           tx.Users,
			Credentials:     tx.Credentials,
			LoginIdentities: tx.LoginIdentities,
		}

		// 解析用户
		userResult, err := s.userResolver.Resolve(txCtx, repos, prepared)
		if err != nil {
			return err
		}

		// 确保登录身份
		loginIdentityResult, err := s.loginIdentityEnsurer.Ensure(txCtx, repos.LoginIdentities, prepared, userResult.User.ID)
		if err != nil {
			return err
		}

		// 确保凭据
		credentialResult, err := s.credentialEnsurer.Ensure(txCtx, repos.Credentials, loginIdentityResult, prepared)
		if err != nil {
			return err
		}

		// 构建开通执行结果
		out = &onboardingExecutionResult{
			User:          userResult,
			LoginIdentity: loginIdentityResult,
			Credential:    credentialResult,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 构建开通结果
	return buildOnboardingResult(out), nil
}

// buildOnboardingResult 构建开通结果。
func buildOnboardingResult(out *onboardingExecutionResult) *OnboardingResult {
	user := out.User.User
	loginIdentity := out.LoginIdentity.Identity
	var credential *OnboardingCredential
	if out.Credential != nil && out.Credential.Credential != nil {
		credential = &OnboardingCredential{
			ID:   out.Credential.Credential.ID,
			Type: out.Credential.Credential.Type,
		}
	}

	return &OnboardingResult{
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

// isRepositoryNotFound 判断是否是仓储未找到错误
func isRepositoryNotFound(err error) bool {
	return perrors.IsCode(err, code.ErrUserNotFound) ||
		perrors.IsCode(err, code.ErrCredentialNotFound)
}
