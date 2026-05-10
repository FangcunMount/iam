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
	MatchedByLoginIdentity UserMatchMethod = "login_identity"
	MatchedByNone          UserMatchMethod = "none"
)

// UserResolveResult 是用户解析阶段的显式结果。
type UserResolveResult struct {
	User      *userDomain.User
	Status    UserResolveStatus
	MatchedBy UserMatchMethod
}

// IsNewUser 是否新建用户
func (r UserResolveResult) IsNewUser() bool {
	return r.Status == UserCreated
}

// userResolver 用户解析器
type userResolver struct {
	fallbackUserRepo userDomain.Repository
}

// newUserResolver 创建用户解析器
func newUserResolver(fallbackUserRepo userDomain.Repository) *userResolver {
	return &userResolver{fallbackUserRepo: fallbackUserRepo}
}

// Resolve 解析用户
func (r *userResolver) Resolve(
	ctx context.Context,
	repos registrationRepositories,
	req *preparedOnboarding,
) (*UserResolveResult, error) {
	userRepo := repos.Users
	if userRepo == nil {
		userRepo = r.fallbackUserRepo
	}
	if userRepo == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "user repository is not initialized")
	}

	// 如果存在登录身份，则解析登录身份
	if result, matched, err := r.resolveByLoginIdentity(ctx, userRepo, repos, req); err != nil {
		return nil, err
	} else if matched {
		return result, nil
	}

	// 如果以上都不匹配，则创建新用户并返回用户解析结果
	return r.createUser(ctx, userRepo, req)
}

// resolveByLoginIdentity 解析登录身份
func (r *userResolver) resolveByLoginIdentity(ctx context.Context, userRepo userDomain.Repository,
	repos registrationRepositories, req *preparedOnboarding) (*UserResolveResult, bool, error) {
	providerKey := req.LoginIdentity.ProviderKey
	identity, err := repos.LoginIdentities.GetByProviderKey(ctx, providerKey.Provider, providerKey.Realm, providerKey.Identifier)
	if err != nil && !isRepositoryNotFound(err) {
		return nil, true, err
	}

	// 如果登录身份不存在，则根据全局标识符获取登录身份
	if identity == nil && providerKey.GlobalIdentifier != "" {
		identity, err = repos.LoginIdentities.GetByGlobalIdentifier(ctx, providerKey.Provider, providerKey.GlobalIdentifier)
		if err != nil && !isRepositoryNotFound(err) {
			return nil, true, err
		}
	}

	// 如果登录身份不存在，则返回空结果
	if identity == nil {
		return nil, false, nil
	}

	// 加载或修复用户
	result, err := r.loadOrRepairUserForLoginIdentity(ctx, userRepo, identity.UserID, req, MatchedByLoginIdentity)
	if err != nil {
		return nil, true, err
	}
	return result, true, nil
}

// createUser 创建新用户
func (r *userResolver) createUser(
	ctx context.Context,
	repo userDomain.Repository,
	req *preparedOnboarding,
) (*UserResolveResult, error) {
	user, err := userDomain.NewUser(req.User.Name, req.User.Phone, func(u *userDomain.User) {
		if !req.User.Email.IsEmpty() {
			u.Email = req.User.Email
		}
	})
	if err != nil {
		return nil, perrors.WithCode(code.ErrUserBasicInfoInvalid, "failed to create user: %v", err)
	}

	// 创建用户
	if err := repo.Create(ctx, user); err != nil {
		return nil, perrors.WithCode(code.ErrDatabase, "failed to save user: %v", err)
	}

	// 返回用户解析结果
	return &UserResolveResult{
		User:      user,
		Status:    UserCreated,
		MatchedBy: MatchedByNone,
	}, nil
}

// loadOrRepairUserForLoginIdentity 加载或修复用户
func (r *userResolver) loadOrRepairUserForLoginIdentity(
	ctx context.Context,
	repo userDomain.Repository,
	userID meta.ID,
	req *preparedOnboarding,
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
	if !req.LoginIdentity.AllowUserRepair {
		return nil, perrors.WithCode(code.ErrUserNotFound, "login identity user not found: %s", userID.String())
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
	req *preparedOnboarding,
) (*userDomain.User, error) {
	opts := []userDomain.UserOption{userDomain.WithID(userID)}
	if !req.User.Email.IsEmpty() {
		opts = append(opts, userDomain.WithEmail(req.User.Email))
	}
	if nickname := strings.TrimSpace(req.LoginIdentity.Profile["nickname"]); nickname != "" {
		opts = append(opts, userDomain.WithNickname(nickname))
	}

	recovered, err := userDomain.NewUser(req.User.Name, req.User.Phone, opts...)
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
