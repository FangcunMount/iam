package admission

import (
	"errors"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	admissiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// MapError 将 Admission 领域错误映射为稳定的 AuthN/Identity 应用错误契约。
// 非 Admission 错误保持不变，供调用方继续按所属用例处理。
func MapError(err error) error {
	if err == nil {
		return nil
	}

	var denied *admissiondomain.DeniedError
	if errors.As(err, &denied) {
		return mapDeniedDecision(denied.Decision)
	}
	var evaluation *admissiondomain.EvaluationError
	if errors.As(err, &evaluation) {
		return perrors.WrapC(err, code.ErrInternalServerError, "failed to evaluate authentication admission")
	}
	return err
}

func mapDeniedDecision(decision admissiondomain.Decision) error {
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
