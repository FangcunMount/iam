package admission

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	admissiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Require evaluates the current User/LoginIdentity state and maps a denied
// decision to the stable AuthN/Identity application error contract.
func Require(
	ctx context.Context,
	policy admissiondomain.Policy,
	userID meta.ID,
	loginIdentityID meta.ID,
) error {
	if policy == nil {
		return perrors.WithCode(code.ErrInternalServerError, "admission policy is not configured")
	}
	decision, err := policy.Evaluate(ctx, userID, loginIdentityID)
	if err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to evaluate authentication admission")
	}
	if decision.IsAdmitted() {
		return nil
	}
	switch decision.Status {
	case admissiondomain.StatusBlocked:
		return perrors.WithCode(code.ErrUserBlocked, "user is blocked")
	case admissiondomain.StatusDisabled:
		return perrors.WithCode(code.ErrLoginIdentityDisabled, "login identity is disabled")
	case admissiondomain.StatusInactive:
		return perrors.WithCode(code.ErrUserInactive, "user is inactive")
	default:
		return perrors.WithCode(code.ErrUserInactive, "authentication admission denied")
	}
}
