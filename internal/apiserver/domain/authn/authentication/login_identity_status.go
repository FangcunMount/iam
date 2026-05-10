package authentication

import (
	"context"
	"fmt"

	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

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
