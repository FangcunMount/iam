package onboarding

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// CredentialEnsureStatus 表示凭据确保阶段的结果。
type CredentialEnsureStatus string

const (
	CredentialCreated     CredentialEnsureStatus = "created"      // 创建
	CredentialReused      CredentialEnsureStatus = "reused"       // 重用
	CredentialNotRequired CredentialEnsureStatus = "not_required" // 不需要
)

// CredentialEnsureResult 凭据确保阶段的结果
type CredentialEnsureResult struct {
	Credential *credDomain.Credential
	Status     CredentialEnsureStatus
}

// HasCredential 是否存在凭据
func (r CredentialEnsureResult) HasCredential() bool {
	return r.Credential != nil && !r.Credential.ID.IsZero()
}

// credentialEnsurer 凭据确保者
type credentialEnsurer struct {
	hasher authentication.PasswordHasher
}

// newCredentialEnsurer 创建凭据确保者
func newCredentialEnsurer(hasher authentication.PasswordHasher) *credentialEnsurer {
	return &credentialEnsurer{hasher: hasher}
}

// Ensure 确保凭据，创建或重用凭据
func (e *credentialEnsurer) Ensure(ctx context.Context, repo credDomain.Repository,
	loginIdentityResult *LoginIdentityEnsureResult, req *preparedOnboarding) (*CredentialEnsureResult, error) {

	if req.LoginIdentity.NeedPasswordCredential {
		return e.ensurePasswordCredential(ctx, repo, loginIdentityResult, req)
	}
	return &CredentialEnsureResult{
		Credential: nil,
		Status:     CredentialNotRequired,
	}, nil
}

// ensurePasswordCredential 确保密码凭据
func (e *credentialEnsurer) ensurePasswordCredential(ctx context.Context, repo credDomain.Repository, loginIdentityResult *LoginIdentityEnsureResult, req *preparedOnboarding) (*CredentialEnsureResult, error) {
	// 若密码凭据已存在，则重用现有凭据
	credential, err := e.findExistingCredential(ctx, repo, loginIdentityResult.Identity.ID)
	if err != nil {
		return nil, err
	}
	if credential != nil {
		return &CredentialEnsureResult{
			Credential: credential,
			Status:     CredentialReused,
		}, nil
	}

	// 若密码凭据不存在，则颁发新凭据
	issuer := credDomain.NewPasswordIssuer(e.hasher)
	credential, err = e.issuePasswordCredential(issuer, loginIdentityResult.Identity.ID, req)
	if err != nil {
		return nil, err
	}

	// 保存凭据
	if err := repo.Create(ctx, credential); err != nil {
		return nil, perrors.WithCode(code.ErrDatabase, "failed to save credential: %v", err)
	}

	// 返回凭据确保结果
	return &CredentialEnsureResult{
		Credential: credential,
		Status:     CredentialCreated,
	}, nil
}

// issuePasswordCredential 颁发密码凭据
func (e *credentialEnsurer) issuePasswordCredential(
	issuer credDomain.CredentialIssuer,
	loginIdentityID meta.ID,
	req *preparedOnboarding,
) (*credDomain.Credential, error) {
	// 如果凭据不存在或密码为空，则无法颁发
	if req.Credential == nil || req.Credential.Password == nil || req.Credential.Password.Plaintext == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "password is required")
	}

	// 颁发密码凭据
	return issuer.IssuePasswordCredential(credDomain.PasswordCredentialRequest{
		LoginIdentityID: loginIdentityID,
		PlainPassword:   req.Credential.Password.Plaintext,
	})
}

// 查找现有凭据
func (e *credentialEnsurer) findExistingCredential(
	ctx context.Context,
	repo credDomain.Repository,
	loginIdentityID meta.ID,
) (*credDomain.Credential, error) {
	credential, err := repo.GetByLoginIdentityIDAndType(ctx, loginIdentityID, credDomain.CredPassword)
	if err != nil {
		if perrors.IsCode(err, code.ErrCredentialNotFound) {
			return nil, nil
		}
		return nil, perrors.WithCode(code.ErrDatabase, "failed to find credential: %v", err)
	}
	return credential, err
}
