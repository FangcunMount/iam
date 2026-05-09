package authentication

import (
	"context"
	"fmt"

	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

func loginIdentityStatusFailureDecision(ctx context.Context, identityRepo LoginIdentityRepository, loginIdentityID meta.ID) (*AuthDecision, error) {
	enabled, locked, err := identityRepo.GetLoginIdentityStatus(ctx, loginIdentityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get login identity status: %w", err)
	}
	if !enabled {
		return &AuthDecision{
			OK:              false,
			Code:            code.ErrCredentialDisabled,
			LoginIdentityID: loginIdentityID,
		}, nil
	}
	if locked {
		return &AuthDecision{
			OK:              false,
			Code:            code.ErrCredentialLocked,
			LoginIdentityID: loginIdentityID,
		}, nil
	}
	return nil, nil
}
