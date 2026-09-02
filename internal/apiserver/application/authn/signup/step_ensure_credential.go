package signup

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	credDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// CredentialEnsureStatus 凭据确保状态。
type CredentialEnsureStatus string

const (
	CredentialCreated     CredentialEnsureStatus = "created"
	CredentialReused      CredentialEnsureStatus = "reused"
	CredentialNotRequired CredentialEnsureStatus = "not_required"
)

// ensureCredentialStepResult 凭据确保步骤结果。
type ensureCredentialStepResult struct {
	Credential *credDomain.Credential
	Status     CredentialEnsureStatus
}

// HasCredential 判断是否存在凭据。
func (r ensureCredentialStepResult) HasCredential() bool {
	return r.Credential != nil && !r.Credential.ID.IsZero()
}

// ensureCredentialStep 凭据确保步骤。
type ensureCredentialStep struct {
	hasher authentication.PasswordHasher
}

// newEnsureCredentialStep 创建凭据确保步骤。
func newEnsureCredentialStep(hasher authentication.PasswordHasher) *ensureCredentialStep {
	return &ensureCredentialStep{hasher: hasher}
}

// Run 执行凭据确保步骤。
func (s *ensureCredentialStep) Run(
	ctx context.Context,
	repo credDomain.Repository,
	loginIdentityResult *ensureLoginIdentityStepResult,
	req *preparedSignup,
) (*ensureCredentialStepResult, error) {
	if req.LoginIdentity.NeedPasswordCredential {
		return s.ensurePasswordCredential(ctx, repo, loginIdentityResult, req)
	}
	return &ensureCredentialStepResult{
		Credential: nil,
		Status:     CredentialNotRequired,
	}, nil
}

// ensurePasswordCredential 确保密码凭据。
func (s *ensureCredentialStep) ensurePasswordCredential(
	ctx context.Context,
	repo credDomain.Repository,
	loginIdentityResult *ensureLoginIdentityStepResult,
	req *preparedSignup,
) (*ensureCredentialStepResult, error) {
	credential, err := s.findExistingCredential(ctx, repo, loginIdentityResult.Identity.ID)
	if err != nil {
		return nil, err
	}
	if credential != nil {
		return &ensureCredentialStepResult{
			Credential: credential,
			Status:     CredentialReused,
		}, nil
	}

	issuer := credDomain.NewPasswordIssuer(s.hasher)
	credential, err = s.issuePasswordCredential(issuer, loginIdentityResult.Identity.ID, req)
	if err != nil {
		return nil, err
	}

	if err := repo.Create(ctx, credential); err != nil {
		return nil, perrors.WithCode(code.ErrDatabase, "failed to save credential: %v", err)
	}

	return &ensureCredentialStepResult{
		Credential: credential,
		Status:     CredentialCreated,
	}, nil
}

// issuePasswordCredential 颁发密码凭据。
func (s *ensureCredentialStep) issuePasswordCredential(
	issuer passwordCredentialIssuer,
	loginIdentityID meta.ID,
	req *preparedSignup,
) (*credDomain.Credential, error) {
	if req.Credential == nil || req.Credential.Password == nil || req.Credential.Password.Plaintext == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "password is required")
	}

	return issuer.IssuePasswordCredential(credDomain.PasswordCredentialRequest{
		LoginIdentityID: loginIdentityID,
		PlainPassword:   req.Credential.Password.Plaintext,
	})
}

// findExistingCredential 查找现有凭据。
func (s *ensureCredentialStep) findExistingCredential(
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

type passwordCredentialIssuer interface {
	IssuePasswordCredential(req credDomain.PasswordCredentialRequest) (*credDomain.Credential, error)
}
