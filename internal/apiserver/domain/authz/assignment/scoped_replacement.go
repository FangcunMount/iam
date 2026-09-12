package assignment

import (
	"slices"
	"sort"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v5/internal/pkg/code"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
)

// ScopedRoleGrant keeps the role and its range together through execution.
type ScopedRoleGrant struct {
	RoleName role.Name
	Scope    scope.Scope
}

type ScopedReplacementRequest struct {
	OrgID            meta.ID
	ManagedRoleNames []string
	Targets          []ScopedRoleGrant
}

type ScopedReplacementPlan struct {
	Grants  []ScopedRoleGrant
	Revokes []AssignmentID
	Changed bool
}

// PlanScoped changes only managed assignments in the requested company. Scope
// changes revoke the old fact and create a new one, preserving historical audit.
func (ReplacementPolicy) PlanScoped(request ScopedReplacementRequest, managedRoles []ManagedRoleBinding, current []*Assignment) (ScopedReplacementPlan, error) {
	invalid := func(message string) (ScopedReplacementPlan, error) {
		return ScopedReplacementPlan{}, perrors.WithCode(code.ErrInvalidArgument, "%s", message)
	}
	if request.OrgID <= 0 {
		return invalid("replacement company is required")
	}
	names, err := normalizeReplacementRoleNames(request.ManagedRoleNames, false)
	if err != nil {
		return ScopedReplacementPlan{}, err
	}
	managed := map[string]bool{}
	for _, name := range names {
		managed[name] = true
	}
	byID := map[meta.ID]string{}
	resolved := map[string]bool{}
	for _, binding := range managedRoles {
		name := binding.Name.String()
		if !managed[name] {
			continue
		}
		if binding.ID <= 0 || resolved[name] || byID[binding.ID] != "" {
			return invalid("ambiguous managed role binding")
		}
		byID[binding.ID] = name
		resolved[name] = true
	}
	if len(resolved) != len(managed) {
		return invalid("managed roles must be resolved")
	}
	targets := map[string]ScopedRoleGrant{}
	for _, target := range request.Targets {
		name := target.RoleName.String()
		if !managed[name] {
			return ScopedReplacementPlan{}, perrors.WithCode(code.ErrPermissionDenied, "role is outside managed assignment set: %s", name)
		}
		if _, duplicate := targets[name]; duplicate {
			return invalid("duplicate target role")
		}
		if target.Scope.IsZero() || target.Scope.OrgID() != request.OrgID {
			return invalid("target scope must belong to replacement company")
		}
		targets[name] = target
	}
	existing := map[string]*Assignment{}
	for _, a := range current {
		if a == nil {
			continue
		}
		name, ok := byID[a.RoleID]
		if !ok {
			continue
		}
		value, configured := a.Scope()
		if !configured {
			return invalid("unconfigured managed assignment requires explicit migration")
		}
		if value.OrgID() != request.OrgID {
			continue
		}
		if a.ID.Uint64() == 0 || existing[name] != nil {
			return invalid("invalid current company assignment")
		}
		existing[name] = a
	}
	plan := ScopedReplacementPlan{}
	for name, a := range existing {
		target, keep := targets[name]
		value, _ := a.Scope()
		if keep && value.Kind() == target.Scope.Kind() && slices.Equal(value.StoreIDs(), target.Scope.StoreIDs()) {
			delete(targets, name)
			continue
		}
		plan.Revokes = append(plan.Revokes, a.ID)
	}
	for _, target := range targets {
		plan.Grants = append(plan.Grants, target)
	}
	sort.Slice(plan.Revokes, func(i, j int) bool { return plan.Revokes[i].Uint64() < plan.Revokes[j].Uint64() })
	sort.Slice(plan.Grants, func(i, j int) bool { return plan.Grants[i].RoleName.String() < plan.Grants[j].RoleName.String() })
	plan.Changed = len(plan.Grants) > 0 || len(plan.Revokes) > 0
	return plan, nil
}
