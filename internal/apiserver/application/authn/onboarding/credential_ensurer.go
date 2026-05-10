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
	CredentialCreated  CredentialEnsureStatus = "created"
	CredentialReused   CredentialEnsureStatus = "reused"
	CredentialConflict CredentialEnsureStatus = "conflict"
	CredentialRotated  CredentialEnsureStatus = "rotated"
)

// CredentialEnsureResult 是凭据确保阶段的显式结果。
type CredentialEnsureResult struct {
	Credential *credDomain.Credential
	Status     CredentialEnsureStatus
}

func (r CredentialEnsureResult) IsNewCredential() bool {
	return r.Status == CredentialCreated
}

type credentialEnsurer struct {
	hasher authentication.PasswordHasher
}

func newCredentialEnsurer(hasher authentication.PasswordHasher) *credentialEnsurer {
	return &credentialEnsurer{hasher: hasher}
}

func (e *credentialEnsurer) Ensure(
	ctx context.Context,
	repo credDomain.Repository,
	loginIdentityResult *LoginIdentityEnsureResult,
	req *NormalizedOnboardingRequest,
) (*CredentialEnsureResult, error) {
	if !req.Plan.NeedCredential {
		return &CredentialEnsureResult{
			Credential: &credDomain.Credential{},
			Status:     CredentialReused,
		}, nil
	}
	issuer := credDomain.NewPasswordIssuer(e.hasher)
	credential, err := e.issuePasswordCredential(issuer, loginIdentityResult.Identity.ID, req)
	if err != nil {
		return nil, err
	}

	if err := repo.Create(ctx, credential); err != nil {
		if perrors.IsCode(err, code.ErrCredentialExists) {
			return e.reuseExisting(ctx, repo, loginIdentityResult.Identity.ID, req)
		}
		return nil, perrors.WithCode(code.ErrDatabase, "failed to save credential: %v", err)
	}

	return &CredentialEnsureResult{
		Credential: credential,
		Status:     CredentialCreated,
	}, nil
}

func (e *credentialEnsurer) issuePasswordCredential(
	issuer *credDomain.PasswordIssuer,
	loginIdentityID meta.ID,
	req *NormalizedOnboardingRequest,
) (*credDomain.Credential, error) {
	if req.Password == nil || *req.Password == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "password is required")
	}
	return issuer.IssuePassword(credDomain.IssuePasswordRequest{
		LoginIdentityID: loginIdentityID,
		PlainPassword:   *req.Password,
	})
}

func (e *credentialEnsurer) reuseExisting(
	ctx context.Context,
	repo credDomain.Repository,
	loginIdentityID meta.ID,
	req *NormalizedOnboardingRequest,
) (*CredentialEnsureResult, error) {
	existing, err := repo.GetByLoginIdentityIDAndType(ctx, loginIdentityID, credDomain.CredPassword)
	if err != nil {
		return nil, perrors.WithCode(code.ErrDatabase, "failed to reuse credential: %v", err)
	}
	return &CredentialEnsureResult{
		Credential: existing,
		Status:     CredentialReused,
	}, nil
}
