package onboarding

import (
	"context"
	"fmt"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	profileLinkDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/profilelink"
	userDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

type userResolution struct {
	User      *userDomain.User
	IsNewUser bool
	Request   OnboardingRequest
}

// UserProvisioner 负责开通流程中的用户供给、复用和缺失用户修复。
type UserProvisioner struct {
	fallbackUserRepo       userDomain.Repository
	wechatIdentityResolver *wechatIdentityResolver
}

func newUserProvisioner(
	fallbackUserRepo userDomain.Repository,
	wechatIdentityResolver *wechatIdentityResolver,
) *UserProvisioner {
	return &UserProvisioner{
		fallbackUserRepo:       fallbackUserRepo,
		wechatIdentityResolver: wechatIdentityResolver,
	}
}

func (r *UserProvisioner) Provision(ctx context.Context, repos registrationRepositories, req OnboardingRequest) (*userResolution, error) {
	userRepo := repos.Users
	if userRepo == nil {
		userRepo = r.fallbackUserRepo
	}

	identity, err := r.resolveWechatIdentity(ctx, req)
	if err != nil {
		return nil, err
	}
	preparedReq := prepareWechatIdentity(req, identity)

	if !preparedReq.ExistingUserID.IsZero() {
		user, err := userRepo.FindByID(ctx, preparedReq.ExistingUserID)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "existing user not found: %s", preparedReq.ExistingUserID.String())
		}
		if err := profileLinkDomain.NewSelfProfileEnsurer(repos.Profiles, repos.ProfileLinks).Ensure(ctx, user); err != nil {
			return nil, err
		}
		return &userResolution{User: user, IsNewUser: false, Request: preparedReq}, nil
	}

	user, isNewUser, err := r.createOrGetUser(ctx, userRepo, repos.Accounts, preparedReq, identity.OpenID, identity.UnionID)
	if err != nil {
		return nil, err
	}
	if err := profileLinkDomain.NewSelfProfileEnsurer(repos.Profiles, repos.ProfileLinks).Ensure(ctx, user); err != nil {
		return nil, err
	}
	return &userResolution{User: user, IsNewUser: isNewUser, Request: preparedReq}, nil
}

func (r *UserProvisioner) createOrGetUser(
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

func (r *UserProvisioner) loadOrRepairUserForAccount(
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

func (r *UserProvisioner) resolveWechatIdentity(ctx context.Context, req OnboardingRequest) (wechatIdentity, error) {
	if r == nil || r.wechatIdentityResolver == nil {
		return wechatIdentity{}, nil
	}
	return r.wechatIdentityResolver.ResolveMiniProgram(ctx, req)
}
