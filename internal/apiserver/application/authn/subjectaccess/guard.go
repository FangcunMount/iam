package subjectaccess

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// RequireAllowed evaluates the current User/LoginIdentity state and maps a
// denied decision to the stable AuthN/Identity application error contract.
func RequireAllowed(
	ctx context.Context,
	evaluator sessiondomain.SubjectAccessEvaluator,
	userID meta.ID,
	loginIdentityID meta.ID,
) error {
	if evaluator == nil {
		return perrors.WithCode(code.ErrInternalServerError, "subject access evaluator is not configured")
	}
	decision, err := evaluator.Evaluate(ctx, userID, loginIdentityID)
	if err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to evaluate subject access")
	}
	if decision.IsAllowed() {
		return nil
	}
	switch decision.Status {
	case sessiondomain.SubjectAccessBlocked:
		return perrors.WithCode(code.ErrUserBlocked, "user is blocked")
	case sessiondomain.SubjectAccessDisabled:
		return perrors.WithCode(code.ErrLoginIdentityDisabled, "login identity is disabled")
	case sessiondomain.SubjectAccessInactive:
		return perrors.WithCode(code.ErrUserInactive, "user is inactive")
	default:
		return perrors.WithCode(code.ErrUserInactive, "subject is inactive")
	}
}
