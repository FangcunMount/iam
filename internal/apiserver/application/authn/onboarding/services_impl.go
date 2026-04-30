package onboarding

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/authn/uow"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	credDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/credential"
	idpPort "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	profileDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/profile"
	profileLinkDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/profilelink"
	userDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// ============= AccountOnboarder 实现 =============

type accountOnboarder struct {
	uow                uow.UnitOfWork
	userProvisioner    *UserProvisioner
	accountProvisioner *AccountProvisioner
	credentialBinder   *CredentialBinder
}

type registrationRepositories struct {
	Users        userDomain.Repository
	Profiles     profileDomain.Repository
	ProfileLinks profileLinkDomain.Repository
	Accounts     domain.Repository
	Credentials  credDomain.Repository
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
		uow:                uow,
		userProvisioner:    newUserProvisioner(userRepo, wechatIdentityResolver),
		accountProvisioner: newAccountProvisioner(),
		credentialBinder:   newCredentialBinder(hasher),
	}
}

// Onboard 统一账号开通接口（使用领域层策略模式 + 凭据绑定分离）。
func (s *accountOnboarder) Onboard(ctx context.Context, req OnboardingRequest) (*OnboardingResult, error) {
	l := logger.L(ctx)
	var result *OnboardingResult

	if req.AccountType == domain.TypeOpera && req.ScopedTenantID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "scoped_tenant_id is required for opera account")
	}
	if req.AccountType != domain.TypeOpera && !req.ScopedTenantID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "scoped_tenant_id is only valid for opera account")
	}

	l.Debugw("开始账号开通流程",
		"action", logger.ActionRegister,
		"resource", logger.ResourceUser,
		"account_type", string(req.AccountType),
		"credential_type", string(req.CredentialType),
		"phone", req.Phone.String(),
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		repos := registrationRepositories{
			Users:        tx.Users,
			Profiles:     tx.Profiles,
			ProfileLinks: tx.ProfileLinks,
			Accounts:     tx.Accounts,
			Credentials:  tx.Credentials,
		}

		l.Debugw("步骤1: 创建或获取用户",
			"action", logger.ActionRegister,
			"phone", req.Phone.String(),
		)
		userResolution, err := s.userProvisioner.Provision(txCtx, repos, req)
		if err != nil {
			l.Errorw("创建或获取用户失败",
				"action", logger.ActionRegister,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}
		user := userResolution.User
		preparedReq := userResolution.Request

		l.Debugw("用户处理完成",
			"action", logger.ActionRegister,
			"user_id", user.ID.String(),
			"is_new_user", userResolution.IsNewUser,
		)

		l.Debugw("步骤2: 创建账户",
			"action", logger.ActionRegister,
			"account_type", string(req.AccountType),
			"user_id", user.ID.String(),
		)
		accountCreation, err := s.accountProvisioner.Provision(txCtx, repos.Accounts, preparedReq, user.ID)
		if err != nil {
			l.Errorw("创建账户失败",
				"action", logger.ActionRegister,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}
		account := accountCreation.Account

		l.Debugw("账户处理完成",
			"action", logger.ActionRegister,
			"account_id", account.ID.String(),
			"account_type", string(account.Type),
			"is_new_account", accountCreation.IsNewAccount,
		)

		l.Debugw("步骤3: 颁发凭据",
			"action", logger.ActionRegister,
			"credential_type", string(req.CredentialType),
			"account_id", account.ID.String(),
		)
		credential, err := s.credentialBinder.Bind(txCtx, repos.Credentials, account.ID, accountCreation.CreationParams, preparedReq)
		if err != nil {
			l.Errorw("颁发凭据失败",
				"action", logger.ActionRegister,
				"credential_type", string(req.CredentialType),
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		idpType := "password"
		if credential.IDP != nil {
			idpType = *credential.IDP
		}
		l.Debugw("凭据颁发完成",
			"action", logger.ActionRegister,
			"credential_id", credential.ID.String(),
			"credential_type", idpType,
		)

		// ========== 步骤4: 构造返回结果 ==========
		result = &OnboardingResult{
			// 用户信息
			UserID:     user.ID,
			UserName:   user.Name,
			Phone:      user.Phone,
			Email:      user.Email,
			UserStatus: user.Status,

			// 账户信息
			AccountID:   account.ID,
			AccountType: account.Type,
			ExternalID:  account.ExternalID,

			// 凭据信息
			CredentialID: credential.ID,
			// 状态
			IsNewUser:    userResolution.IsNewUser,
			IsNewAccount: accountCreation.IsNewAccount,
		}

		return nil
	})

	if err != nil {
		l.Errorw("用户注册失败",
			"action", logger.ActionRegister,
			"resource", logger.ResourceUser,
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
		return nil, err
	}

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

// mapCredentialType 将应用层凭据类型映射为领域层类型
func mapCredentialType(t CredentialType) credDomain.CredentialType {
	switch t {
	case CredTypePhone:
		return credDomain.CredPhoneOTP
	case CredTypeWechat:
		return credDomain.CredOAuthWxMinip
	case CredTypeWecom:
		return credDomain.CredOAuthWecom
	default:
		return credDomain.CredPassword
	}
}

func isRepositoryNotFound(err error) bool {
	return perrors.IsCode(err, code.ErrUserNotFound) ||
		perrors.IsCode(err, code.ErrCredentialNotFound) ||
		perrors.IsCode(err, code.ErrNotFoundAccount)
}
