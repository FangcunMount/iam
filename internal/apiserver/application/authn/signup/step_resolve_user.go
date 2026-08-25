package signup

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	userDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

type UserResolveStatus string

const (
	UserCreated  UserResolveStatus = "created"
	UserReused   UserResolveStatus = "reused"
	UserRepaired UserResolveStatus = "repaired"
)

type UserMatchMethod string

const (
	MatchedByLoginIdentity UserMatchMethod = "login_identity"
	MatchedByNone          UserMatchMethod = "none"
)

type resolveUserStepResult struct {
	User      *userDomain.User
	Status    UserResolveStatus
	MatchedBy UserMatchMethod
}

func (r resolveUserStepResult) IsNewUser() bool {
	return r.Status == UserCreated
}

type resolveUserStep struct {
	fallbackUserRepo userDomain.Repository
}

func newResolveUserStep(fallbackUserRepo userDomain.Repository) *resolveUserStep {
	return &resolveUserStep{fallbackUserRepo: fallbackUserRepo}
}

func (s *resolveUserStep) Run(
	ctx context.Context,
	repos registrationRepositories,
	req *preparedSignup,
) (*resolveUserStepResult, error) {
	userRepo := repos.Users
	if userRepo == nil {
		userRepo = s.fallbackUserRepo
	}
	if userRepo == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "user repository is not initialized")
	}

	if result, matched, err := s.resolveByLoginIdentity(ctx, userRepo, repos, req); err != nil {
		return nil, err
	} else if matched {
		return result, nil
	}

	return s.createUser(ctx, userRepo, req)
}

func (s *resolveUserStep) resolveByLoginIdentity(
	ctx context.Context,
	userRepo userDomain.Repository,
	repos registrationRepositories,
	req *preparedSignup,
) (*resolveUserStepResult, bool, error) {
	providerKey := req.LoginIdentity.ProviderKey
	identity, err := repos.LoginIdentities.GetByProviderKey(ctx, providerKey.Provider(), providerKey.Realm(), providerKey.Identifier())
	if err != nil && !isRepositoryNotFound(err) {
		return nil, true, err
	}

	if identity == nil && providerKey.GlobalIdentifier() != "" {
		identity, err = repos.LoginIdentities.GetByGlobalIdentifier(ctx, providerKey.Provider(), providerKey.GlobalIdentifier())
		if err != nil && !isRepositoryNotFound(err) {
			return nil, true, err
		}
	}

	if identity == nil {
		return nil, false, nil
	}

	result, err := s.loadOrRepairUserForLoginIdentity(ctx, userRepo, identity.UserID, req, MatchedByLoginIdentity)
	if err != nil {
		return nil, true, err
	}
	return result, true, nil
}

func (s *resolveUserStep) createUser(
	ctx context.Context,
	repo userDomain.Repository,
	req *preparedSignup,
) (*resolveUserStepResult, error) {
	user, err := userDomain.NewUser(req.User.Name, req.User.Phone, func(u *userDomain.User) {
		if !req.User.Email.IsEmpty() {
			u.Email = req.User.Email
		}
	})
	if err != nil {
		return nil, perrors.WithCode(code.ErrUserBasicInfoInvalid, "failed to create user: %v", err)
	}

	if err := repo.Create(ctx, user); err != nil {
		return nil, perrors.WithCode(code.ErrDatabase, "failed to save user: %v", err)
	}

	return &resolveUserStepResult{
		User:      user,
		Status:    UserCreated,
		MatchedBy: MatchedByNone,
	}, nil
}

func (s *resolveUserStep) loadOrRepairUserForLoginIdentity(
	ctx context.Context,
	repo userDomain.Repository,
	userID meta.ID,
	req *preparedSignup,
	matchedBy UserMatchMethod,
) (*resolveUserStepResult, error) {
	user, err := repo.FindByID(ctx, userID)
	if err == nil && user != nil {
		return &resolveUserStepResult{
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

	recovered, err := s.repairMissingUser(ctx, repo, userID, req)
	if err != nil {
		return nil, err
	}
	return &resolveUserStepResult{
		User:      recovered,
		Status:    UserRepaired,
		MatchedBy: matchedBy,
	}, nil
}

func (s *resolveUserStep) repairMissingUser(
	ctx context.Context,
	repo userDomain.Repository,
	userID meta.ID,
	req *preparedSignup,
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
