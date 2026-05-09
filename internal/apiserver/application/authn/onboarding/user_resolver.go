package onboarding

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// UserResolveStatus 表示用户解析阶段的结果。
type UserResolveStatus string

const (
	UserCreated  UserResolveStatus = "created"
	UserReused   UserResolveStatus = "reused"
	UserRepaired UserResolveStatus = "repaired"
)

// UserMatchMethod 表示用户解析命中的依据。
type UserMatchMethod string

const (
	MatchedByExistingUserID UserMatchMethod = "existing_user_id"
	MatchedByWechatUnionID  UserMatchMethod = "wechat_union_id"
	MatchedByWechatOpenID   UserMatchMethod = "wechat_openid_appid"
	MatchedByPhone          UserMatchMethod = "phone"
	MatchedByNone           UserMatchMethod = "none"
)

// UserResolveResult 是用户解析阶段的显式结果。
type UserResolveResult struct {
	User      *userDomain.User
	Status    UserResolveStatus
	MatchedBy UserMatchMethod
}

func (r UserResolveResult) IsNewUser() bool {
	return r.Status == UserCreated
}

type userResolver struct {
	fallbackUserRepo userDomain.Repository
}

func newUserResolver(fallbackUserRepo userDomain.Repository) *userResolver {
	return &userResolver{fallbackUserRepo: fallbackUserRepo}
}

func (r *userResolver) Resolve(
	ctx context.Context,
	repos registrationRepositories,
	req *NormalizedOnboardingRequest,
) (*UserResolveResult, error) {
	userRepo := repos.Users
	if userRepo == nil {
		userRepo = r.fallbackUserRepo
	}
	if userRepo == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "user repository is not initialized")
	}

	if !req.ExistingUserID.IsZero() {
		return r.resolveExistingUser(ctx, userRepo, req.ExistingUserID)
	}
	if result, matched, err := req.strategy.ResolveUserByAccount(ctx, r, userRepo, repos.Accounts, req); matched || err != nil {
		return result, err
	}
	if result, matched, err := r.resolveByPhone(ctx, userRepo, req); matched || err != nil {
		return result, err
	}
	return r.createUser(ctx, userRepo, req)
}

func (r *userResolver) resolveExistingUser(
	ctx context.Context,
	repo userDomain.Repository,
	userID meta.ID,
) (*UserResolveResult, error) {
	user, err := repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "existing user not found: %s", userID.String())
	}
	return &UserResolveResult{
		User:      user,
		Status:    UserReused,
		MatchedBy: MatchedByExistingUserID,
	}, nil
}

func (r *userResolver) resolveByPhone(
	ctx context.Context,
	repo userDomain.Repository,
	req *NormalizedOnboardingRequest,
) (*UserResolveResult, bool, error) {
	if req.Phone.IsEmpty() {
		return nil, false, nil
	}
	existingUser, err := repo.FindByPhone(ctx, req.Phone)
	if err != nil && !isRepositoryNotFound(err) {
		return nil, true, err
	}
	if existingUser == nil {
		return nil, false, nil
	}
	return &UserResolveResult{
		User:      existingUser,
		Status:    UserReused,
		MatchedBy: MatchedByPhone,
	}, true, nil
}

func (r *userResolver) createUser(
	ctx context.Context,
	repo userDomain.Repository,
	req *NormalizedOnboardingRequest,
) (*UserResolveResult, error) {
	user, err := userDomain.NewUser(req.Name, req.Phone, func(u *userDomain.User) {
		if !req.Email.IsEmpty() {
			u.Email = req.Email
		}
	})
	if err != nil {
		return nil, perrors.WithCode(code.ErrUserBasicInfoInvalid, "failed to create user: %v", err)
	}
	if err := repo.Create(ctx, user); err != nil {
		return nil, perrors.WithCode(code.ErrDatabase, "failed to save user: %v", err)
	}
	return &UserResolveResult{
		User:      user,
		Status:    UserCreated,
		MatchedBy: MatchedByNone,
	}, nil
}

func (r *userResolver) loadOrRepairUserForAccount(
	ctx context.Context,
	repo userDomain.Repository,
	userID meta.ID,
	req *NormalizedOnboardingRequest,
	matchedBy UserMatchMethod,
) (*UserResolveResult, error) {
	user, err := repo.FindByID(ctx, userID)
	if err == nil && user != nil {
		return &UserResolveResult{
			User:      user,
			Status:    UserReused,
			MatchedBy: matchedBy,
		}, nil
	}
	if err != nil && !isRepositoryNotFound(err) {
		return nil, err
	}
	if !req.Plan.AllowUserRepair {
		return nil, perrors.WithCode(code.ErrUserNotFound, "account user not found: %s", userID.String())
	}

	recovered, err := r.repairMissingUser(ctx, repo, userID, req)
	if err != nil {
		return nil, err
	}
	return &UserResolveResult{
		User:      recovered,
		Status:    UserRepaired,
		MatchedBy: matchedBy,
	}, nil
}

func (r *userResolver) repairMissingUser(
	ctx context.Context,
	repo userDomain.Repository,
	userID meta.ID,
	req *NormalizedOnboardingRequest,
) (*userDomain.User, error) {
	opts := []userDomain.UserOption{userDomain.WithID(userID)}
	if !req.Email.IsEmpty() {
		opts = append(opts, userDomain.WithEmail(req.Email))
	}
	if nickname := strings.TrimSpace(req.Profile["nickname"]); nickname != "" {
		opts = append(opts, userDomain.WithNickname(nickname))
	}

	recovered, err := userDomain.NewUser(req.Name, req.Phone, opts...)
	if err != nil {
		return nil, perrors.WithCode(code.ErrUserBasicInfoInvalid, "failed to recreate missing user: %v", err)
	}
	if err = repo.Create(ctx, recovered); err != nil {
		if perrors.IsCode(err, code.ErrUserAlreadyExists) {
			existing, getErr := repo.FindByID(ctx, userID)
			if getErr != nil {
				return nil, getErr
			}
			return existing, nil
		}
		return nil, perrors.WithCode(code.ErrDatabase, "failed to recreate missing user: %v", err)
	}
	return recovered, nil
}

func valueOfStringPtr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
