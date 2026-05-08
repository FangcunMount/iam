package authentication

import (
	"context"
	"fmt"

	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// accountStatusFailureDecision 检查账户状态是否失败
func accountStatusFailureDecision(ctx context.Context, accountRepo AccountRepository, accountID meta.ID) (*AuthDecision, error) {
	enabled, locked, err := accountRepo.GetAccountStatus(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account status: %w", err)
	}
	if !enabled {
		return &AuthDecision{
			OK:        false,
			Code:      code.ErrCredentialDisabled,
			AccountID: accountID,
		}, nil
	}
	if locked {
		return &AuthDecision{
			OK:        false,
			Code:      code.ErrCredentialLocked,
			AccountID: accountID,
		}, nil
	}

	return nil, nil
}
