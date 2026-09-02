package assignment

import (
	"sort"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// ReplacementPlan describes the managed assignment changes to apply atomically.
type ReplacementPlan struct {
	Grants      []roleDomain.Name
	Revokes     []AssignmentID
	DirectRoles []roleDomain.Name
	Changed     bool
}

// ReplacementRequest carries the caller intent before role identifiers are resolved.
type ReplacementRequest struct {
	TargetRoleNames  []string
	ManagedRoleNames []string
}

// ManagedRoleBinding maps a managed role name to its persisted role identifier.
type ManagedRoleBinding struct {
	Name roleDomain.Name
	ID   meta.ID
}

// ReplacementPolicy computes managed assignment replacement plans.
type ReplacementPolicy struct{}

// Plan derives the grant/revoke operations required to reach the target managed
// role set while preserving unmanaged assignments.
func (ReplacementPolicy) Plan(
	request ReplacementRequest,
	managedRoles []ManagedRoleBinding,
	currentAssignments []*Assignment,
) (ReplacementPlan, error) {
	targets, err := normalizeReplacementRoleNames(request.TargetRoleNames, true)
	if err != nil {
		return ReplacementPlan{}, err
	}
	managedNames, err := normalizeReplacementRoleNames(request.ManagedRoleNames, false)
	if err != nil {
		return ReplacementPlan{}, err
	}
	managedSet := make(map[string]struct{}, len(managedNames))
	for _, roleName := range managedNames {
		managedSet[roleName] = struct{}{}
	}
	for _, roleName := range targets {
		if _, ok := managedSet[roleName]; !ok {
			return ReplacementPlan{}, perrors.WithCode(code.ErrPermissionDenied, "role is outside the managed assignment set: %s", roleName)
		}
	}

	targetSet := make(map[string]struct{}, len(targets))
	directRoles := make([]roleDomain.Name, 0, len(targets))
	for _, roleName := range targets {
		targetSet[roleName] = struct{}{}
		name, err := roleDomain.NewName(roleName)
		if err != nil {
			return ReplacementPlan{}, err
		}
		directRoles = append(directRoles, name)
	}

	managedByID := make(map[uint64]string, len(managedRoles))
	for _, binding := range managedRoles {
		managedByID[binding.ID.Uint64()] = binding.Name.String()
	}
	currentManaged := make(map[string]*Assignment)
	for _, assignment := range currentAssignments {
		if assignment == nil {
			continue
		}
		if roleName, ok := managedByID[assignment.RoleID.Uint64()]; ok {
			currentManaged[roleName] = assignment
		}
	}

	plan := ReplacementPlan{DirectRoles: directRoles}
	revokeIDs := make([]AssignmentID, 0)
	for roleName, assignment := range currentManaged {
		if _, keep := targetSet[roleName]; keep {
			continue
		}
		revokeIDs = append(revokeIDs, assignment.ID)
		plan.Changed = true
	}
	sort.Slice(revokeIDs, func(i, j int) bool { return revokeIDs[i].Uint64() < revokeIDs[j].Uint64() })
	plan.Revokes = revokeIDs

	grants := make([]roleDomain.Name, 0, len(targets))
	for _, roleName := range targets {
		if _, exists := currentManaged[roleName]; exists {
			continue
		}
		name, err := roleDomain.NewName(roleName)
		if err != nil {
			return ReplacementPlan{}, err
		}
		grants = append(grants, name)
		plan.Changed = true
	}
	sort.Slice(grants, func(i, j int) bool { return grants[i].String() < grants[j].String() })
	plan.Grants = grants
	return plan, nil
}

func normalizeReplacementRoleNames(values []string, allowEmpty bool) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		roleName, err := roleDomain.NewName(value)
		if err != nil {
			return nil, err
		}
		name := roleName.String()
		if _, exists := seen[name]; exists {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "duplicate role name: %s", name)
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	if !allowEmpty && len(result) == 0 {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "managed role names are required")
	}
	sort.Strings(result)
	return result, nil
}
