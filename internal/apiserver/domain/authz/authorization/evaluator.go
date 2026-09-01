// Package authorization owns the domain language and policies used to produce
// an authorization decision from immutable authorization facts.
package authorization

import (
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// EvaluationContext contains the immutable authorization facts selected for a
// single request. It is a derived value, not a persisted authorization fact.
type EvaluationContext struct {
	EffectiveRoles []role.Name
	GrantsByRole   map[role.Name][]*permissiongrant.Grant
	Resource       *resource.Resource
	PolicyVersion  int64
}

// Evaluator applies PermissionGrant and ConstraintSet domain rules to an
// authorization request. It has no persistence or runtime-snapshot concerns.
type Evaluator struct{}

func NewEvaluator() Evaluator { return Evaluator{} }

func (Evaluator) Evaluate(
	request Request,
	context EvaluationContext,
	evaluatedAt time.Time,
) (Decision, error) {
	if context.Resource != nil {
		if err := ValidateAttributes(context.Resource.AttributeSchema, request.Object.Attributes); err != nil {
			return Decision{}, err
		}
	} else if len(request.Object.Attributes) > 0 {
		return Decision{}, perrors.WithCode(
			code.ErrInvalidArgument,
			"object attributes require a registered resource",
		)
	}

	missing := make([]string, 0)
	for _, roleName := range context.EffectiveRoles {
		for _, grant := range context.GrantsByRole[roleName] {
			if grant == nil {
				return Decision{}, perrors.WithCode(
					code.ErrInvalidArgument,
					"authorization candidate grant is required",
				)
			}
			if !grant.CoversResource(request.ResourceKey) || !grant.MatchesAction(request.Action) {
				continue
			}
			evaluation, err := grant.Evaluate(request.Object.Attributes)
			if err != nil {
				return Decision{}, err
			}
			if evaluation.Matched {
				return Allow(grant.ID, roleName.String(), context.PolicyVersion, evaluatedAt), nil
			}
			missing = append(missing, evaluation.MissingAttributeKeys...)
		}
	}
	return Deny(context.PolicyVersion, missing, evaluatedAt), nil
}
