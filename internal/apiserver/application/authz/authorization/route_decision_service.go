package authorization

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authorizationdomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// RoutePermissionChecker decides whether a subject may perform an action on a
// route resource. It does not execute HTTP control flow.
type RoutePermissionChecker interface {
	CheckRoutePermission(ctx context.Context, subjectKey, tenantID, resourceKey, action string) (bool, error)
}

// RouteDecisionService translates route-level permission questions into the
// canonical authorization request handled by DecisionService.
type RouteDecisionService struct {
	decisions *DecisionService
}

func NewRouteDecisionService(decisions *DecisionService) *RouteDecisionService {
	return &RouteDecisionService{decisions: decisions}
}

func (s *RouteDecisionService) CheckRoutePermission(
	ctx context.Context,
	subjectKey, tenantID, resourceKey, action string,
) (bool, error) {
	if s == nil || s.decisions == nil {
		return false, perrors.WithCode(code.ErrInternalServerError, "authorization decision service is unavailable")
	}
	sub, err := subject.ParseRef(subjectKey)
	if err != nil {
		return false, err
	}
	object, err := authorizationdomain.NewObjectContext("", nil)
	if err != nil {
		return false, err
	}
	request, err := authorizationdomain.NewRequest(sub, tenantID, resourceKey, action, object)
	if err != nil {
		return false, err
	}
	decision, err := s.decisions.Check(ctx, request)
	return decision.Allowed, err
}
