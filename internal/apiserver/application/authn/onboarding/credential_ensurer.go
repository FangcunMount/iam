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
	accountResult *AccountEnsureResult,
	req *NormalizedOnboardingRequest,
) (*CredentialEnsureResult, error) {
	issuer := credDomain.NewIssuer(e.hasher)
	credential, err := req.strategy.IssueCredential(ctx, issuer, accountResult.Account.ID, accountResult.CreationParams, req)
	if err != nil {
		return nil, err
	}

	if err := repo.Create(ctx, credential); err != nil {
		if perrors.IsCode(err, code.ErrCredentialExists) {
			return e.reuseExisting(ctx, repo, accountResult.Account.ID, req)
		}
		return nil, perrors.WithCode(code.ErrDatabase, "failed to save credential: %v", err)
	}

	return &CredentialEnsureResult{
		Credential: credential,
		Status:     CredentialCreated,
	}, nil
}

func (e *credentialEnsurer) reuseExisting(
	ctx context.Context,
	repo credDomain.Repository,
	accountID meta.ID,
	req *NormalizedOnboardingRequest,
) (*CredentialEnsureResult, error) {
	credType := req.strategy.CredentialRepositoryType()
	existing, err := repo.GetByAccountIDAndType(ctx, accountID, credType)
	if err != nil {
		return nil, perrors.WithCode(code.ErrDatabase, "failed to reuse credential: %v", err)
	}
	return &CredentialEnsureResult{
		Credential: existing,
		Status:     CredentialReused,
	}, nil
}
