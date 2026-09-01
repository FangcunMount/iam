package authorization

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authorizationdomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// DecisionRuntime supplies the active authorization policy used by Checker.
type DecisionRuntime interface {
	Check(context.Context, authorizationdomain.Request) (authorizationdomain.Decision, error)
}

// Checker is the application capability used to execute one authorization
// request against the active runtime policy.
type Checker struct {
	runtime DecisionRuntime
}

func NewChecker(runtime DecisionRuntime) *Checker { return &Checker{runtime: runtime} }

func (c *Checker) Check(ctx context.Context, request authorizationdomain.Request) (authorizationdomain.Decision, error) {
	if c == nil || c.runtime == nil {
		return authorizationdomain.Decision{}, perrors.WithCode(code.ErrInternalServerError, "authorization runtime is unavailable")
	}
	return c.runtime.Check(ctx, request)
}
