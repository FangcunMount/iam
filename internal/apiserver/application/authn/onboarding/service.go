package onboarding

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/uow"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	loginidentityDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	profileDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
	profileLinkDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profilelink"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	idpPort "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

type loginIdentityOnboarder struct {
	uow                  uow.UnitOfWork
	normalizer           *requestNormalizer
	userResolver         *userResolver
	loginIdentityEnsurer *loginIdentityEnsurer
	credentialEnsurer    *credentialEnsurer
}

type registrationRepositories struct {
	Users           userDomain.Repository
	Profiles        profileDomain.Repository
	ProfileLinks    profileLinkDomain.Repository
	Credentials     credDomain.Repository
	LoginIdentities loginidentityDomain.Repository
}

type onboardingExecutionResult struct {
	User          *UserResolveResult
	LoginIdentity *LoginIdentityEnsureResult
	Credential    *CredentialEnsureResult
}

var _ LoginIdentityOnboarder = (*loginIdentityOnboarder)(nil)

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
		normalizer:           newRequestNormalizer(wechatIdentityResolver),
		userResolver:         newUserResolver(userRepo),
		loginIdentityEnsurer: newLoginIdentityEnsurer(),
		credentialEnsurer:    newCredentialEnsurer(hasher),
	}
}

// Onboard 统一登录身份开通接口。
func (s *loginIdentityOnboarder) Onboard(ctx context.Context, req OnboardingRequest) (*OnboardingResult, error) {
	l := logger.L(ctx)

	normalized, err := s.normalizer.Normalize(ctx, req)
	if err != nil {
		l.Errorw("登录身份开通请求归一化失败",
			"action", logger.ActionRegister,
			"resource", logger.ResourceUser,
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return nil, err
	}

	l.Debugw("开始登录身份开通流程",
		"action", logger.ActionRegister,
		"resource", logger.ResourceUser,
		"scenario", string(normalized.Plan.Scenario),
		"phone", normalized.Phone.String(),
	)

	var out *onboardingExecutionResult
	err = s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		repos := newRegistrationRepositories(tx)

		userResult, err := s.userResolver.Resolve(txCtx, repos, normalized)
		if err != nil {
			return err
		}
		l.Debugw("用户解析完成",
			"action", logger.ActionRegister,
			"user_id", userResult.User.ID.String(),
			"user_resolve_status", string(userResult.Status),
			"user_matched_by", string(userResult.MatchedBy),
		)

		loginIdentityResult, err := s.loginIdentityEnsurer.Ensure(txCtx, repos.LoginIdentities, normalized, userResult.User.ID)
		if err != nil {
			return err
		}
		l.Debugw("登录身份确保完成",
			"action", logger.ActionRegister,
			"login_identity_id", loginIdentityResult.Identity.ID.String(),
			"provider", string(loginIdentityResult.Identity.Provider),
			"realm", loginIdentityResult.Identity.Realm,
			"login_identity_ensure_status", string(loginIdentityResult.Status),
		)

		credentialResult, err := s.credentialEnsurer.Ensure(txCtx, repos.Credentials, loginIdentityResult, normalized)
		if err != nil {
			return err
		}
		l.Debugw("凭据确保完成",
			"action", logger.ActionRegister,
			"credential_id", credentialResult.Credential.ID.String(),
			"credential_ensure_status", string(credentialResult.Status),
		)

		out = &onboardingExecutionResult{
			User:          userResult,
			LoginIdentity: loginIdentityResult,
			Credential:    credentialResult,
		}
		return nil
	})
	if err != nil {
		l.Errorw("登录身份开通失败",
			"action", logger.ActionRegister,
			"resource", logger.ResourceUser,
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return nil, err
	}

	result := buildOnboardingResult(out)
	l.Debugw("登录身份开通成功",
		"action", logger.ActionRegister,
		"resource", logger.ResourceUser,
		"user_id", result.UserID.String(),
		"login_identity_id", result.LoginIdentityID.String(),
		"credential_id", result.CredentialID.String(),
		"is_new_user", result.IsNewUser,
		"is_new_login_identity", result.IsNewLoginIdentity,
		"result", logger.ResultSuccess,
	)
	return result, nil
}

func newRegistrationRepositories(tx uow.TxRepositories) registrationRepositories {
	return registrationRepositories{
		Users:           tx.Users,
		Profiles:        tx.Profiles,
		ProfileLinks:    tx.ProfileLinks,
		Credentials:     tx.Credentials,
		LoginIdentities: tx.LoginIdentities,
	}
}

func buildOnboardingResult(out *onboardingExecutionResult) *OnboardingResult {
	user := out.User.User
	loginIdentity := out.LoginIdentity.Identity
	credential := out.Credential.Credential

	return &OnboardingResult{
		UserID:             user.ID,
		UserName:           user.Name,
		Phone:              user.Phone,
		Email:              user.Email,
		UserStatus:         user.Status,
		LoginIdentityID:    loginIdentity.ID,
		CredentialID:       credential.ID,
		IsNewUser:          out.User.IsNewUser(),
		IsNewLoginIdentity: out.LoginIdentity.IsNewLoginIdentity(),
	}
}

func isRepositoryNotFound(err error) bool {
	return perrors.IsCode(err, code.ErrUserNotFound) ||
		perrors.IsCode(err, code.ErrCredentialNotFound)
}
