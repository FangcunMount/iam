package token

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	admissiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// requireAdmission 将领域准入判定收敛为 Token 生命周期使用的稳定错误语义。
func requireAdmission(ctx context.Context, policy AdmissionPolicy, subject admissiondomain.Subject) error {
	if policy == nil {
		return perrors.WithCode(code.ErrInternalServerError, "admission policy is not configured")
	}
	decision, err := policy.Evaluate(ctx, subject)
	if err != nil {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to evaluate authentication admission")
	}
	if decision.IsAdmitted() {
		return nil
	}
	switch decision.Reason {
	case admissiondomain.ReasonIdentityOwnerMismatch,
		admissiondomain.ReasonUserMissing,
		admissiondomain.ReasonUserBlocked:
		return perrors.WithCode(code.ErrUserBlocked, "user is blocked")
	case admissiondomain.ReasonLoginIdentityMissing,
		admissiondomain.ReasonLoginIdentityDisabled:
		return perrors.WithCode(code.ErrLoginIdentityDisabled, "login identity is disabled")
	case admissiondomain.ReasonUserInactive:
		return perrors.WithCode(code.ErrUserInactive, "user is inactive")
	default:
		return perrors.WithCode(code.ErrUserInactive, "authentication admission denied")
	}
}
