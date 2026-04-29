package register

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/authn/uow"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	credDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/credential"
	idpPort "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	userDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// ============= RegisterApplicationService 实现 =============

type registerApplicationService struct {
	uow              uow.UnitOfWork
	userResolver     *UserResolver
	accountCreator   *AccountCreator
	credentialIssuer *CredentialIssuer
}

type registrationRepositories struct {
	Users       userDomain.Repository
	Accounts    domain.Repository
	Credentials credDomain.Repository
}

var _ RegisterApplicationService = (*registerApplicationService)(nil)

func NewRegisterApplicationService(
	uow uow.UnitOfWork,
	hasher authentication.PasswordHasher,
	idp authentication.IdentityProvider,
	userRepo userDomain.Repository,
	wechatAppQuerier idpPort.Repository,
	secretVault idpPort.SecretVault,
) RegisterApplicationService {
	return &registerApplicationService{
		uow:              uow,
		userResolver:     newUserResolver(userRepo, idp, wechatAppQuerier, secretVault),
		accountCreator:   newAccountCreator(idp, wechatAppQuerier, secretVault),
		credentialIssuer: newCredentialIssuer(hasher),
	}
}

// Register 统一注册接口（使用领域层策略模式 + 凭据绑定分离）
func (s *registerApplicationService) Register(ctx context.Context, req RegisterRequest) (*RegisterResult, error) {
	l := logger.L(ctx)
	var result *RegisterResult

	if req.AccountType == domain.TypeOpera && req.ScopedTenantID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "scoped_tenant_id is required for opera account")
	}
	if req.AccountType != domain.TypeOpera && !req.ScopedTenantID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "scoped_tenant_id is only valid for opera account")
	}

	l.Debugw("开始用户注册流程",
		"action", logger.ActionRegister,
		"resource", logger.ResourceUser,
		"account_type", string(req.AccountType),
		"credential_type", string(req.CredentialType),
		"phone", req.Phone.String(),
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		repos := registrationRepositories{
			Users:       tx.Users,
			Accounts:    tx.Accounts,
			Credentials: tx.Credentials,
		}

		l.Debugw("步骤1: 创建或获取用户",
			"action", logger.ActionRegister,
			"phone", req.Phone.String(),
		)
		userResolution, err := s.userResolver.Resolve(ctx, repos, &req)
		if err != nil {
			l.Errorw("创建或获取用户失败",
				"action", logger.ActionRegister,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}
		user := userResolution.User

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
		accountCreation, err := s.accountCreator.Create(ctx, repos.Accounts, req, user.ID)
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
		credential, err := s.credentialIssuer.Issue(ctx, repos.Credentials, account.ID, accountCreation.CreationParams, req)
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
		result = &RegisterResult{
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

	l.Debugw("用户注册成功",
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
