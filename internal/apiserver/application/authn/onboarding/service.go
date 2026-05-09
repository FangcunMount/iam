package onboarding

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/uow"
	accountDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	profileDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
	profileLinkDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profilelink"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	idpPort "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

type accountOnboarder struct {
	uow               uow.UnitOfWork
	normalizer        *requestNormalizer
	userResolver      *userResolver
	accountEnsurer    *accountEnsurer
	credentialEnsurer *credentialEnsurer
}

type registrationRepositories struct {
	Users        userDomain.Repository
	Profiles     profileDomain.Repository
	ProfileLinks profileLinkDomain.Repository
	Accounts     accountDomain.Repository
	Credentials  credDomain.Repository
}

type onboardingExecutionResult struct {
	User       *UserResolveResult
	Account    *AccountEnsureResult
	Credential *CredentialEnsureResult
}

var _ AccountOnboarder = (*accountOnboarder)(nil)

func NewAccountOnboarder(
	uow uow.UnitOfWork,
	hasher authentication.PasswordHasher,
	idp authentication.IdentityProvider,
	userRepo userDomain.Repository,
	wechatAppQuerier idpPort.Repository,
	secretVault idpPort.SecretVault,
) AccountOnboarder {
	wechatIdentityResolver := newWechatIdentityResolver(idp, wechatAppQuerier, secretVault)
	return &accountOnboarder{
		uow:               uow,
		normalizer:        newRequestNormalizer(wechatIdentityResolver),
		userResolver:      newUserResolver(userRepo),
		accountEnsurer:    newAccountEnsurer(),
		credentialEnsurer: newCredentialEnsurer(hasher),
	}
}

// Onboard 统一账号开通接口。
func (s *accountOnboarder) Onboard(ctx context.Context, req OnboardingRequest) (*OnboardingResult, error) {
	l := logger.L(ctx)

	normalized, err := s.normalizer.Normalize(ctx, req)
	if err != nil {
		l.Errorw("账号开通请求归一化失败",
			"action", logger.ActionRegister,
			"resource", logger.ResourceUser,
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return nil, err
	}

	l.Debugw("开始账号开通流程",
		"action", logger.ActionRegister,
		"resource", logger.ResourceUser,
		"scenario", string(normalized.Plan.Scenario),
		"account_type", string(normalized.Plan.AccountType),
		"credential_type", string(normalized.Plan.CredentialType),
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

		accountResult, err := s.accountEnsurer.Ensure(txCtx, repos.Accounts, normalized, userResult.User.ID)
		if err != nil {
			return err
		}
		l.Debugw("账号确保完成",
			"action", logger.ActionRegister,
			"account_id", accountResult.Account.ID.String(),
			"account_type", string(accountResult.Account.Type),
			"account_ensure_status", string(accountResult.Status),
		)

		credentialResult, err := s.credentialEnsurer.Ensure(txCtx, repos.Credentials, accountResult, normalized)
		if err != nil {
			return err
		}
		l.Debugw("凭据确保完成",
			"action", logger.ActionRegister,
			"credential_id", credentialResult.Credential.ID.String(),
			"credential_ensure_status", string(credentialResult.Status),
		)

		out = &onboardingExecutionResult{
			User:       userResult,
			Account:    accountResult,
			Credential: credentialResult,
		}
		return nil
	})
	if err != nil {
		l.Errorw("账号开通失败",
			"action", logger.ActionRegister,
			"resource", logger.ResourceUser,
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return nil, err
	}

	result := buildOnboardingResult(out)
	l.Debugw("账号开通成功",
		"action", logger.ActionRegister,
		"resource", logger.ResourceUser,
		"user_id", result.UserID.String(),
		"account_id", result.AccountID.String(),
		"credential_id", result.CredentialID.String(),
		"is_new_user", result.IsNewUser,
		"is_new_account", result.IsNewAccount,
		"result", logger.ResultSuccess,
	)
	return result, nil
}

func newRegistrationRepositories(tx uow.TxRepositories) registrationRepositories {
	return registrationRepositories{
		Users:        tx.Users,
		Profiles:     tx.Profiles,
		ProfileLinks: tx.ProfileLinks,
		Accounts:     tx.Accounts,
		Credentials:  tx.Credentials,
	}
}

func buildOnboardingResult(out *onboardingExecutionResult) *OnboardingResult {
	user := out.User.User
	account := out.Account.Account
	credential := out.Credential.Credential

	return &OnboardingResult{
		UserID:       user.ID,
		UserName:     user.Name,
		Phone:        user.Phone,
		Email:        user.Email,
		UserStatus:   user.Status,
		AccountID:    account.ID,
		AccountType:  account.Type,
		ExternalID:   account.ExternalID,
		CredentialID: credential.ID,
		IsNewUser:    out.User.IsNewUser(),
		IsNewAccount: out.Account.IsNewAccount(),
	}
}

func isRepositoryNotFound(err error) bool {
	return perrors.IsCode(err, code.ErrUserNotFound) ||
		perrors.IsCode(err, code.ErrCredentialNotFound) ||
		perrors.IsCode(err, code.ErrNotFoundAccount)
}
