package authorization

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authorizationdomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// DecisionRuntime supplies the active authorization policy used by DecisionService.
type DecisionRuntime interface {
	Check(context.Context, authorizationdomain.Request) (authorizationdomain.Decision, error)
}

// DecisionService is the application capability used to execute one authorization
// request against the active runtime policy.
type DecisionService struct {
	runtime DecisionRuntime
}

func NewDecisionService(runtime DecisionRuntime) *DecisionService {
	return &DecisionService{runtime: runtime}
}

func (s *DecisionService) Check(ctx context.Context, request authorizationdomain.Request) (authorizationdomain.Decision, error) {
	if s == nil || s.runtime == nil {
		return authorizationdomain.Decision{}, perrors.WithCode(code.ErrInternalServerError, "authorization runtime is unavailable")
	}
	return s.runtime.Check(ctx, request)
}
