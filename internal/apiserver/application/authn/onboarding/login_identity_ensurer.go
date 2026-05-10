package onboarding

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// LoginIdentityEnsureStatus 登录身份确保状态。
type LoginIdentityEnsureStatus string

const (
	LoginIdentityCreated LoginIdentityEnsureStatus = "created" // 创建
	LoginIdentityReused  LoginIdentityEnsureStatus = "reused"  // 重用
)

// LoginIdentityEnsureResult 登录身份确保结果。
type LoginIdentityEnsureResult struct {
	Identity *loginidentity.LoginIdentity // 登录身份
	Status   LoginIdentityEnsureStatus    // 确保状态
}

// IsNewLoginIdentity 是否是新登录身份。
func (r LoginIdentityEnsureResult) IsNewLoginIdentity() bool {
	return r.Status == LoginIdentityCreated
}

// loginIdentityEnsurer 登录身份确保者。
type loginIdentityEnsurer struct{}

// newLoginIdentityEnsurer 创建登录身份确保者。
func newLoginIdentityEnsurer() *loginIdentityEnsurer { return &loginIdentityEnsurer{} }

// Ensure 确保 ProviderKey 对应的 LoginIdentity 存在，并保护身份归属不被跨 User 复用。
func (e *loginIdentityEnsurer) Ensure(ctx context.Context, repo loginidentity.Repository, req *preparedOnboarding, userID meta.ID) (*LoginIdentityEnsureResult, error) {
	// 构建领域登录身份。
	identity, err := buildDomainIdentity(req, userID)
	if err != nil {
		return nil, err
	}

	// 若登录身份已存在，则复用现有登录身份。
	if existing, err := findExistingLoginIdentity(ctx, repo, identity); err != nil {
		return nil, err
	} else if existing != nil {
		return &LoginIdentityEnsureResult{Identity: existing, Status: LoginIdentityReused}, nil
	}

	// 若登录身份不存在，则创建新登录身份。
	if created, err := createLoginIdentity(ctx, repo, identity); err != nil {
		return nil, err
	} else {
		return &LoginIdentityEnsureResult{Identity: created, Status: LoginIdentityCreated}, nil
	}
}

// buildDomainIdentity 构建领域登录身份。
func buildDomainIdentity(req *preparedOnboarding, userID meta.ID) (*loginidentity.LoginIdentity, error) {
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
	if existing != nil {
		if existing.UserID != identity.UserID {
			return nil, perrors.WithCode(code.ErrLoginIdentityExists, "login identity already belongs to another user")
		}
		if !existing.IsActive() {
			return nil, perrors.WithCode(code.ErrLoginIdentityDisabled, "login identity is not active")
		}
		return existing, nil
	}
	return nil, nil
}

// createLoginIdentity 创建登录身份。
func createLoginIdentity(ctx context.Context, repo loginidentity.Repository, identity *loginidentity.LoginIdentity) (*loginidentity.LoginIdentity, error) {
	if err := repo.Create(ctx, identity); err != nil {
		return nil, perrors.WithCode(code.ErrDatabase, "failed to save login identity: %v", err)
	}
	return identity, nil
}
