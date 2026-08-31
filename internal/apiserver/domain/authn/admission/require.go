package admission

import (
	"context"
	"errors"
	"fmt"
)

// DeniedError 表示认证主体未通过领域准入判定。
type DeniedError struct {
	Decision Decision
}

func (e *DeniedError) Error() string {
	if e == nil {
		return "authentication admission denied"
	}
	return fmt.Sprintf("authentication admission denied: %s", e.Decision.Reason)
}

// EvaluationError 表示认证准入策略无法完成判定。
type EvaluationError struct {
	Err error
}

func (e *EvaluationError) Error() string {
	if e == nil || e.Err == nil {
		return "failed to evaluate authentication admission"
	}
	return fmt.Sprintf("failed to evaluate authentication admission: %v", e.Err)
}

func (e *EvaluationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Require 要求认证主体通过准入判定，否则返回可由应用层统一映射的领域错误。
func Require(ctx context.Context, policy Policy, subject Subject) error {
	if policy == nil {
		return &EvaluationError{Err: errors.New("admission policy is not configured")}
	}

	decision, err := policy.Evaluate(ctx, subject)
	if err != nil {
		return &EvaluationError{Err: err}
	}
	if decision.IsAdmitted() {
		return nil
	}
	return &DeniedError{Decision: decision}
}
