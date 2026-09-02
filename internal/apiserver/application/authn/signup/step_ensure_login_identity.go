package signup

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// LoginIdentityEnsureStatus 登录身份确保状态。
type LoginIdentityEnsureStatus string

const (
	LoginIdentityCreated LoginIdentityEnsureStatus = "created"
	LoginIdentityReused  LoginIdentityEnsureStatus = "reused"
)

// ensureLoginIdentityStepResult 登录身份确保步骤结果。
type ensureLoginIdentityStepResult struct {
	Identity *loginidentity.LoginIdentity
	Status   LoginIdentityEnsureStatus
}

// IsNewLoginIdentity 判断是否是新的登录身份。
func (r ensureLoginIdentityStepResult) IsNewLoginIdentity() bool {
	return r.Status == LoginIdentityCreated
}

// ensureLoginIdentityStep 登录身份确保步骤。
type ensureLoginIdentityStep struct{}

// newEnsureLoginIdentityStep 创建登录身份确保步骤。
func newEnsureLoginIdentityStep() *ensureLoginIdentityStep {
	return &ensureLoginIdentityStep{}
}

// Run 执行登录身份确保步骤。
func (s *ensureLoginIdentityStep) Run(
	ctx context.Context,
	repo loginidentity.Repository,
	req *preparedSignup,
	userID meta.ID,
) (*ensureLoginIdentityStepResult, error) {
	identity, err := buildDomainIdentity(req, userID)
	if err != nil {
		return nil, err
	}

	if existing, err := findExistingLoginIdentity(ctx, repo, identity); err != nil {
		return nil, err
	} else if existing != nil {
		return &ensureLoginIdentityStepResult{Identity: existing, Status: LoginIdentityReused}, nil
	}

	created, err := createLoginIdentity(ctx, repo, identity)
	if err != nil {
		return nil, err
	}
	return &ensureLoginIdentityStepResult{Identity: created, Status: LoginIdentityCreated}, nil
}

// buildDomainIdentity 构建领域登录身份。
func buildDomainIdentity(req *preparedSignup, userID meta.ID) (*loginidentity.LoginIdentity, error) {
	return loginidentity.NewBuilder(userID).
		FromProviderKey(req.LoginIdentity.ProviderKey).
		WithProfile(req.LoginIdentity.Profile).
		WithMeta(req.LoginIdentity.Meta).
		Build()
}

// findExistingLoginIdentity 查找现有登录身份。
func findExistingLoginIdentity(ctx context.Context, repo loginidentity.Repository, identity *loginidentity.LoginIdentity) (*loginidentity.LoginIdentity, error) {
	existing, err := repo.GetByProviderKey(ctx, identity.Provider, identity.Realm, identity.Identifier)
	if err != nil {
		return nil, err
	}
	switch loginidentity.AssessBinding(loginidentity.BindingRequest{
		RequestUserID: identity.UserID,
		Existing:      existing,
	}) {
	case loginidentity.BindingCreate:
		return nil, nil
	case loginidentity.BindingReuse:
		return existing, nil
	case loginidentity.BindingConflictOtherUser:
		return nil, perrors.WithCode(code.ErrLoginIdentityExists, "login identity already belongs to another user")
	case loginidentity.BindingInactiveSameUser:
		return nil, perrors.WithCode(code.ErrLoginIdentityDisabled, "login identity is not active")
	default:
		return nil, perrors.WithCode(code.ErrDatabase, "unexpected login identity binding decision")
	}
}

// createLoginIdentity 创建登录身份。
func createLoginIdentity(ctx context.Context, repo loginidentity.Repository, identity *loginidentity.LoginIdentity) (*loginidentity.LoginIdentity, error) {
	if err := repo.Create(ctx, identity); err != nil {
		if perrors.IsCode(err, code.ErrGlobalIdentifierExists) || perrors.IsCode(err, code.ErrLoginIdentityExists) {
			return nil, err
		}
		return nil, perrors.WithCode(code.ErrDatabase, "failed to save login identity: %v", err)
	}
	return identity, nil
}
