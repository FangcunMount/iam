package register

import (
	"context"
	"fmt"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/authn/uow"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	credDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/credential"
	idpPort "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	userDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"gorm.io/gorm"
)

// ============= RegisterApplicationService 实现 =============

type registerApplicationService struct {
	uow              uow.UnitOfWork
	userRepo         userDomain.Repository
	hasher           authentication.PasswordHasher
	idp              authentication.IdentityProvider
	wechatAppQuerier idpPort.Repository
	secretVault      idpPort.SecretVault
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
		userRepo:         userRepo,
		hasher:           hasher,
		idp:              idp,
		wechatAppQuerier: wechatAppQuerier,
		secretVault:      secretVault,
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

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// ========== 步骤1: 创建或获取 User ==========
		l.Debugw("步骤1: 创建或获取用户",
			"action", logger.ActionRegister,
			"phone", req.Phone.String(),
		)

		userRepo := tx.Users
		if userRepo == nil {
			userRepo = s.userRepo
		}
		accountRepo := tx.Accounts

		var user *userDomain.User
		var isNewUser bool

		if !req.ExistingUserID.IsZero() {
			u, err := userRepo.FindByID(ctx, req.ExistingUserID)
			if err != nil {
				l.Errorw("按 ExistingUserID 加载用户失败",
					"action", logger.ActionRegister,
					"user_id", req.ExistingUserID.String(),
					"error", err.Error(),
					"result", logger.ResultFailed,
				)
				return err
			}
			if u == nil {
				return perrors.WithCode(code.ErrInvalidArgument, "existing user not found: %s", req.ExistingUserID.String())
			}
			user = u
			isNewUser = false
		} else {
			openID, unionID, err := s.resolveWechatIDs(ctx, req)
			if err != nil {
				l.Errorw("解析微信身份失败",
					"action", logger.ActionRegister,
					"error", err.Error(),
					"result", logger.ResultFailed,
				)
				return err
			}
			if openID != "" && req.WechatOpenID == nil {
				req.WechatOpenID = &openID
			}
			if unionID != "" && req.WechatUnionID == nil {
				req.WechatUnionID = &unionID
			}
			if openID != "" && req.WechatJsCode != nil {
				req.WechatJsCode = nil
			}

			var errGet error
			user, isNewUser, errGet = s.createOrGetUser(ctx, userRepo, accountRepo, req, openID, unionID)
			if errGet != nil {
				l.Errorw("创建或获取用户失败",
					"action", logger.ActionRegister,
					"error", errGet.Error(),
					"result", logger.ResultFailed,
				)
				return errGet
			}
		}

		l.Debugw("用户处理完成",
			"action", logger.ActionRegister,
			"user_id", user.ID.String(),
			"is_new_user", isNewUser,
		)

		// ========== 步骤2: 根据 AccountType 创建账户 ==========
		l.Debugw("步骤2: 创建账户",
			"action", logger.ActionRegister,
			"account_type", string(req.AccountType),
			"user_id", user.ID.String(),
		)

		// 构造领域层输入（包含查询 AppSecret）
		domainInput, err := s.toDomainInput(ctx, req, user.ID)
		if err != nil {
			l.Errorw("构造领域输入失败",
				"action", logger.ActionRegister,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 创建账户创建器（策略选择在领域层内部完成）
		accountCreator := domain.NewAccountCreator(tx.Accounts, s.idp)

		// 创建账户实体（不包含持久化）
		account, creationParams, err := accountCreator.CreateAccount(ctx, domainInput)
		if err != nil {
			l.Errorw("创建账户实体失败",
				"action", logger.ActionRegister,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		isNewAccount := false
		// 如果账户是新创建的（不是从数据库查到的），需要持久化
		if account.ID.IsZero() {
			if err := tx.Accounts.Create(ctx, account); err != nil {
				l.Errorw("持久化账户失败",
					"action", logger.ActionRegister,
					"error", err.Error(),
					"result", logger.ResultFailed,
				)
				return perrors.WithCode(code.ErrDatabase, "failed to save account: %v", err)
			}
			isNewAccount = true
		}

		l.Debugw("账户处理完成",
			"action", logger.ActionRegister,
			"account_id", account.ID.String(),
			"account_type", string(account.Type),
			"is_new_account", isNewAccount,
		)

		// ========== 步骤3: 根据 CredentialType 颁发凭据 ==========
		l.Debugw("步骤3: 颁发凭据",
			"action", logger.ActionRegister,
			"credential_type", string(req.CredentialType),
			"account_id", account.ID.String(),
		)

		credIssuer := credDomain.NewIssuer(s.hasher)
		credential, err := s.issueCredential(ctx, credIssuer, account.ID, creationParams, req)
		if err != nil {
			l.Errorw("颁发凭据失败",
				"action", logger.ActionRegister,
				"credential_type", string(req.CredentialType),
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化凭据（幂等：凭据已存在时复用现有凭据）
		if err := tx.Credentials.Create(ctx, credential); err != nil {
			if perrors.IsCode(err, code.ErrCredentialExists) {
				credType := mapCredentialType(req.CredentialType)
				existing, getErr := tx.Credentials.GetByAccountIDAndType(ctx, account.ID, credType)
				if getErr != nil {
					l.Errorw("查询已存在的凭据失败",
						"action", logger.ActionRegister,
						"error", getErr.Error(),
						"result", logger.ResultFailed,
					)
					return perrors.WithCode(code.ErrDatabase, "failed to reuse credential: %v", getErr)
				}
				// 复用已存在的凭据
				credential = existing
			} else {
				l.Errorw("持久化凭据失败",
					"action", logger.ActionRegister,
					"error", err.Error(),
					"result", logger.ResultFailed,
				)
				return perrors.WithCode(code.ErrDatabase, "failed to save credential: %v", err)
			}
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
			IsNewUser:    isNewUser,
			IsNewAccount: isNewAccount,
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

// issueCredential 根据凭据类型颁发凭据
func (s *registerApplicationService) issueCredential(
	ctx context.Context,
	issuer credDomain.Issuer,
	accountID meta.ID,
	creationParams *domain.CreationParams,
	req RegisterRequest,
) (*credDomain.Credential, error) {
	switch req.CredentialType {
	case CredTypePassword:
		// 颁发密码凭据
		if req.Password == nil || *req.Password == "" {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "password is required")
		}
		return issuer.IssuePassword(ctx, credDomain.IssuePasswordRequest{
			AccountID:     accountID,
			PlainPassword: *req.Password,
		})

	case CredTypePhone:
		// 颁发手机OTP凭据
		return issuer.IssuePhoneOTP(ctx, credDomain.IssuePhoneOTPRequest{
			AccountID: accountID,
			Phone:     req.Phone,
		})

	case CredTypeWechat:
		// 颁发微信凭据
		if creationParams == nil || creationParams.OpenID == "" {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "openid is required for wechat credential")
		}
		// 优先使用 UnionID 作为标识符
		idpIdentifier := creationParams.OpenID
		if creationParams.UnionID != "" {
			idpIdentifier = creationParams.UnionID
		}
		appID := ""
		if req.WechatAppID != nil {
			appID = *req.WechatAppID
		}
		return issuer.IssueWechatMinip(ctx, credDomain.IssueOAuthRequest{
			AccountID:     accountID,
			IDPIdentifier: idpIdentifier,
			AppID:         appID,
			ParamsJSON:    req.ParamsJSON,
		})

	case CredTypeWecom:
		// 颁发企业微信凭据
		if req.WecomUserID == nil || *req.WecomUserID == "" {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "wecom userid is required")
		}
		appID := ""
		if req.WecomCorpID != nil {
			appID = *req.WecomCorpID
		}
		return issuer.IssueWecom(ctx, credDomain.IssueOAuthRequest{
			AccountID:     accountID,
			IDPIdentifier: *req.WecomUserID,
			AppID:         appID,
			ParamsJSON:    req.ParamsJSON,
		})

	default:
		return nil, perrors.WithCode(code.ErrInvalidArgument, "unsupported credential type: %s", req.CredentialType)
	}
}

// toDomainInput 将应用层DTO转换为领域层输入，必要时查询 AppSecret
func (s *registerApplicationService) toDomainInput(ctx context.Context, req RegisterRequest, userID meta.ID) (domain.CreationInput, error) {
	input := domain.CreationInput{
		UserID:         userID,
		Phone:          req.Phone,
		Email:          req.Email,
		OperaLoginID:   strings.TrimSpace(req.OperaLoginID),
		ScopedTenantID: req.ScopedTenantID,
		AccountType:    req.AccountType,
		WechatAppID:    req.WechatAppID,
		WechatJsCode:   req.WechatJsCode,
		WechatOpenID:   req.WechatOpenID,
		WechatUnionID:  req.WechatUnionID,
		WecomCorpID:    req.WecomCorpID,
		WecomUserID:    req.WecomUserID,
		Profile:        req.Profile,
		Meta:           req.Meta,
		ParamsJSON:     req.ParamsJSON,
	}

	// 如果是微信小程序注册且提供了 JsCode，需要查询 AppSecret
	if req.AccountType == domain.TypeWcMinip && req.WechatAppID != nil && req.WechatJsCode != nil {
		if s.wechatAppQuerier == nil || s.secretVault == nil {
			return domain.CreationInput{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app configuration service not available")
		}

		wechatApp, err := s.wechatAppQuerier.GetByAppID(ctx, *req.WechatAppID)
		if err != nil {
			return domain.CreationInput{}, perrors.WithCode(code.ErrInvalidArgument, "failed to query wechat app: %v", err)
		}
		if wechatApp == nil {
			return domain.CreationInput{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app not found: %s", *req.WechatAppID)
		}
		if !wechatApp.IsEnabled() {
			return domain.CreationInput{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app is disabled: %s", *req.WechatAppID)
		}
		if wechatApp.Cred == nil || wechatApp.Cred.Auth == nil {
			return domain.CreationInput{}, perrors.WithCode(code.ErrInvalidArgument, "wechat app credentials not found")
		}

		appSecretPlain, err := s.secretVault.Decrypt(ctx, wechatApp.Cred.Auth.AppSecretCipher)
		if err != nil {
			return domain.CreationInput{}, perrors.WithCode(code.ErrInvalidArgument, "failed to decrypt app secret: %v", err)
		}

		appSecret := string(appSecretPlain)
		input.WechatAppSecret = &appSecret
	}

	return input, nil
}

// ============= 内部辅助方法 =============

// createOrGetUser 创建或获取用户（步骤1）
func (s *registerApplicationService) createOrGetUser(
	ctx context.Context,
	repo userDomain.Repository,
	accountRepo domain.Repository,
	req RegisterRequest,
	wechatOpenID string,
	wechatUnionID string,
) (*userDomain.User, bool, error) {
	if repo == nil {
		return nil, false, perrors.WithCode(code.ErrInternalServerError, "user repository is not initialized")
	}

	// 如果是微信小程序注册且提供了 UnionID，则通过 UnionID 查找现有用户
	if req.AccountType == domain.TypeWcMinip && accountRepo != nil {
		// 如果是微信小程序注册且提供了 UnionID，则通过 UnionID 查找现有用户
		if wechatUnionID != "" {
			account, err := accountRepo.GetByUniqueID(ctx, domain.UnionID(wechatUnionID))
			if err != nil && !isRepositoryNotFound(err) {
				return nil, false, err
			}
			if account != nil {
				return s.loadOrRepairUserForAccount(ctx, repo, account.UserID, req)
			}
		}

		// 如果是微信小程序注册且提供了 OpenID，则通过 OpenID 查找现有用户
		if wechatOpenID != "" && req.WechatAppID != nil && *req.WechatAppID != "" {
			externalID := domain.ExternalID(fmt.Sprintf("%s@%s", wechatOpenID, *req.WechatAppID))
			appID := domain.AppId(*req.WechatAppID)
			account, err := accountRepo.GetByExternalIDAppId(ctx, externalID, appID)
			if err != nil && !isRepositoryNotFound(err) {
				return nil, false, err
			}
			if account != nil {
				return s.loadOrRepairUserForAccount(ctx, repo, account.UserID, req)
			}
		}
	}

	// 通过手机号查找现有用户
	if !req.Phone.IsEmpty() {
		existingUser, err := repo.FindByPhone(ctx, req.Phone)
		if err != nil && !isRepositoryNotFound(err) {
			return nil, false, err
		}
		// 用户已存在
		if existingUser != nil {
			return existingUser, false, nil
		}
	}

	// 创建新用户
	user, err := userDomain.NewUser(req.Name, req.Phone, func(u *userDomain.User) {
		if !req.Email.IsEmpty() {
			u.Email = req.Email
		}
	})
	if err != nil {
		return nil, false, perrors.WithCode(code.ErrUserBasicInfoInvalid, "failed to create user: %v", err)
	}

	// 持久化用户
	if err := repo.Create(ctx, user); err != nil {
		return nil, false, perrors.WithCode(code.ErrDatabase, "failed to save user: %v", err)
	}

	return user, true, nil
}

func (s *registerApplicationService) loadOrRepairUserForAccount(
	ctx context.Context,
	repo userDomain.Repository,
	userID meta.ID,
	req RegisterRequest,
) (*userDomain.User, bool, error) {
	user, err := repo.FindByID(ctx, userID)
	if err == nil {
		return user, false, nil
	}
	if !isRepositoryNotFound(err) {
		return nil, false, err
	}

	// Self-heal dangling account -> user references. This can happen when the
	// auth/account side already exists but the corresponding user row was lost.
	opts := []userDomain.UserOption{userDomain.WithID(userID)}
	if !req.Email.IsEmpty() {
		opts = append(opts, userDomain.WithEmail(req.Email))
	}
	if nickname := strings.TrimSpace(req.Profile["nickname"]); nickname != "" {
		opts = append(opts, userDomain.WithNickname(nickname))
	}
	recovered, createErr := userDomain.NewUser(req.Name, req.Phone, opts...)
	if createErr != nil {
		return nil, false, perrors.WithCode(code.ErrUserBasicInfoInvalid, "failed to recreate missing user: %v", createErr)
	}
	if createErr = repo.Create(ctx, recovered); createErr != nil {
		if perrors.IsCode(createErr, code.ErrUserAlreadyExists) {
			existing, getErr := repo.FindByID(ctx, userID)
			if getErr != nil {
				return nil, false, getErr
			}
			return existing, false, nil
		}
		return nil, false, perrors.WithCode(code.ErrDatabase, "failed to recreate missing user: %v", createErr)
	}
	return recovered, false, nil
}

// resolveWechatIDs 解析微信小程序的 OpenID 和 UnionID
func (s *registerApplicationService) resolveWechatIDs(ctx context.Context, req RegisterRequest) (string, string, error) {
	if req.AccountType != domain.TypeWcMinip {
		return "", "", nil
	}
	if req.WechatOpenID != nil && *req.WechatOpenID != "" {
		openID := *req.WechatOpenID
		unionID := ""
		if req.WechatUnionID != nil {
			unionID = *req.WechatUnionID
		}
		return openID, unionID, nil
	}
	if req.WechatAppID == nil || *req.WechatAppID == "" || req.WechatJsCode == nil || *req.WechatJsCode == "" {
		return "", "", nil
	}
	if s.wechatAppQuerier == nil || s.secretVault == nil {
		return "", "", perrors.WithCode(code.ErrInvalidArgument, "wechat app configuration service not available")
	}

	wechatApp, err := s.wechatAppQuerier.GetByAppID(ctx, *req.WechatAppID)
	if err != nil {
		return "", "", perrors.WithCode(code.ErrInvalidArgument, "failed to query wechat app: %v", err)
	}
	if wechatApp == nil {
		return "", "", perrors.WithCode(code.ErrInvalidArgument, "wechat app not found: %s", *req.WechatAppID)
	}
	if !wechatApp.IsEnabled() {
		return "", "", perrors.WithCode(code.ErrInvalidArgument, "wechat app is disabled: %s", *req.WechatAppID)
	}
	if wechatApp.Cred == nil || wechatApp.Cred.Auth == nil {
		return "", "", perrors.WithCode(code.ErrInvalidArgument, "wechat app credentials not found")
	}

	appSecretPlain, err := s.secretVault.Decrypt(ctx, wechatApp.Cred.Auth.AppSecretCipher)
	if err != nil {
		return "", "", perrors.WithCode(code.ErrInvalidArgument, "failed to decrypt app secret: %v", err)
	}

	openID, unionID, err := s.idp.ExchangeWxMinipCode(ctx, *req.WechatAppID, string(appSecretPlain), *req.WechatJsCode)
	if err != nil {
		return "", "", perrors.WithCode(code.ErrInvalidCredential, "failed to call wechat code2session: %v", err)
	}
	return openID, unionID, nil
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
	return perrors.Is(err, gorm.ErrRecordNotFound) ||
		perrors.IsCode(err, code.ErrUserNotFound) ||
		perrors.IsCode(err, code.ErrCredentialNotFound) ||
		perrors.IsCode(err, code.ErrNotFoundAccount)
}
