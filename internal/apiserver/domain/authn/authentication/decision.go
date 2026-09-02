package authentication

import (
	"context"
	"fmt"

	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// CredentialEffect 描述认证后对长期凭据的副作用。
type CredentialEffect int

const (
	CredentialEffectNone CredentialEffect = iota
	CredentialEffectRecordFailure
	CredentialEffectRecordSuccess
)

// AuthDecision 认证决策。
type AuthDecision struct {
	OK bool

	// Code 表示认证不通过的业务原因；只有 OK=false 时有效。
	Code int

	// Principal 表示认证成功后的主体；只有 OK=true 时有效。
	Principal *Principal

	LoginIdentityID meta.ID
	CredentialID    meta.ID

	CredentialEffect CredentialEffect

	ShouldRotate bool
	NewMaterial  []byte
	NewAlgo      *string
}

func loginIdentityStatusFailureDecision(ctx context.Context, identityRepo LoginIdentityRepository, loginIdentityID meta.ID) (*AuthDecision, error) {
	active, err := identityRepo.IsLoginIdentityActive(ctx, loginIdentityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get login identity status: %w", err)
	}
	if !active {
		return &AuthDecision{
			OK:              false,
			Code:            code.ErrLoginIdentityDisabled,
			LoginIdentityID: loginIdentityID,
		}, nil
	}
	return nil, nil
}
