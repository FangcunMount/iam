package onboarding

import (
	"context"
	"fmt"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	idpPort "github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	profileLinkDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/profilelink"
	userDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

type userResolution struct {
	User      *userDomain.User
	IsNewUser bool
}

// UserResolver 负责注册流程中的用户解析、复用和缺失用户修复。
type UserResolver struct {
	fallbackUserRepo userDomain.Repository
	idp              authentication.IdentityProvider
	wechatAppQuerier idpPort.Repository
	secretVault      idpPort.SecretVault
}

func newUserResolver(
	fallbackUserRepo userDomain.Repository,
	idp authentication.IdentityProvider,
	wechatAppQuerier idpPort.Repository,
	secretVault idpPort.SecretVault,
) *UserResolver {
	return &UserResolver{
		fallbackUserRepo: fallbackUserRepo,
		idp:              idp,
		wechatAppQuerier: wechatAppQuerier,
		secretVault:      secretVault,
	}
}

func (r *UserResolver) Resolve(ctx context.Context, repos registrationRepositories, req *OnboardingRequest) (*userResolution, error) {
	userRepo := repos.Users
	if userRepo == nil {
		userRepo = r.fallbackUserRepo
	}

	if !req.ExistingUserID.IsZero() {
		user, err := userRepo.FindByID(ctx, req.ExistingUserID)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "existing user not found: %s", req.ExistingUserID.String())
		}
		if err := profileLinkDomain.NewSelfProfileEnsurer(repos.Profiles, repos.ProfileLinks).Ensure(ctx, user); err != nil {
			return nil, err
		}
		return &userResolution{User: user, IsNewUser: false}, nil
	}

	openID, unionID, err := r.resolveWechatIDs(ctx, *req)
	if err != nil {
		return nil, err
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

	user, isNewUser, err := r.createOrGetUser(ctx, userRepo, repos.Accounts, *req, openID, unionID)
	if err != nil {
		return nil, err
	}
	if err := profileLinkDomain.NewSelfProfileEnsurer(repos.Profiles, repos.ProfileLinks).Ensure(ctx, user); err != nil {
		return nil, err
	}
	return &userResolution{User: user, IsNewUser: isNewUser}, nil
}

func (r *UserResolver) createOrGetUser(
	ctx context.Context,
	repo userDomain.Repository,
	accountRepo domain.Repository,
	req OnboardingRequest,
	wechatOpenID string,
	wechatUnionID string,
) (*userDomain.User, bool, error) {
	if repo == nil {
		return nil, false, perrors.WithCode(code.ErrInternalServerError, "user repository is not initialized")
	}

	if req.AccountType == domain.TypeWcMinip && accountRepo != nil {
		if wechatUnionID != "" {
			account, err := accountRepo.GetByUniqueID(ctx, domain.UnionID(wechatUnionID))
			if err != nil && !isRepositoryNotFound(err) {
				return nil, false, err
			}
			if account != nil {
				return r.loadOrRepairUserForAccount(ctx, repo, account.UserID, req)
			}
		}

		if wechatOpenID != "" && req.WechatAppID != nil && *req.WechatAppID != "" {
			externalID := domain.ExternalID(fmt.Sprintf("%s@%s", wechatOpenID, *req.WechatAppID))
			appID := domain.AppId(*req.WechatAppID)
			account, err := accountRepo.GetByExternalIDAppId(ctx, externalID, appID)
			if err != nil && !isRepositoryNotFound(err) {
				return nil, false, err
			}
			if account != nil {
				return r.loadOrRepairUserForAccount(ctx, repo, account.UserID, req)
			}
		}
	}

	if !req.Phone.IsEmpty() {
		existingUser, err := repo.FindByPhone(ctx, req.Phone)
		if err != nil && !isRepositoryNotFound(err) {
			return nil, false, err
		}
		if existingUser != nil {
			return existingUser, false, nil
		}
	}

	user, err := userDomain.NewUser(req.Name, req.Phone, func(u *userDomain.User) {
		if !req.Email.IsEmpty() {
			u.Email = req.Email
		}
	})
	if err != nil {
		return nil, false, perrors.WithCode(code.ErrUserBasicInfoInvalid, "failed to create user: %v", err)
	}

	if err := repo.Create(ctx, user); err != nil {
		return nil, false, perrors.WithCode(code.ErrDatabase, "failed to save user: %v", err)
	}

	return user, true, nil
}

func (r *UserResolver) loadOrRepairUserForAccount(
	ctx context.Context,
	repo userDomain.Repository,
	userID meta.ID,
	req OnboardingRequest,
) (*userDomain.User, bool, error) {
	user, err := repo.FindByID(ctx, userID)
	if err == nil {
		return user, false, nil
	}
	if !isRepositoryNotFound(err) {
		return nil, false, err
	}

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

func (r *UserResolver) resolveWechatIDs(ctx context.Context, req OnboardingRequest) (string, string, error) {
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
	if r.wechatAppQuerier == nil || r.secretVault == nil {
		return "", "", perrors.WithCode(code.ErrInvalidArgument, "wechat app configuration service not available")
	}

	wechatApp, err := r.wechatAppQuerier.GetByAppID(ctx, *req.WechatAppID)
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

	appSecretPlain, err := r.secretVault.Decrypt(ctx, wechatApp.Cred.Auth.AppSecretCipher)
	if err != nil {
		return "", "", perrors.WithCode(code.ErrInvalidArgument, "failed to decrypt app secret: %v", err)
	}

	openID, unionID, err := r.idp.ExchangeWxMinipCode(ctx, *req.WechatAppID, string(appSecretPlain), *req.WechatJsCode)
	if err != nil {
		return "", "", perrors.WithCode(code.ErrInvalidCredential, "failed to call wechat code2session: %v", err)
	}
	return openID, unionID, nil
}
