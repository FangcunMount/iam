package policy

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// RoleRemovalPolicy validates whether a role can be removed safely.
type RoleRemovalPolicy struct{}

// EnsureUnused rejects role removal when active assignments, grants, or
// inheritances still reference the role.
func (RoleRemovalPolicy) EnsureUnused(
	roleID meta.ID,
	assignments []*assignment.Assignment,
	grants []*permissiongrant.Grant,
	inheritances []*roleinheritance.Inheritance,
) error {
	if len(assignments) > 0 {
		return perrors.WithCode(code.ErrRoleInUse, "role has %d active assignments", len(assignments))
	}
	for _, grant := range grants {
		if grant != nil && grant.IsActive() {
			return perrors.WithCode(code.ErrRoleInUse, "role has active permission grants")
		}
	}
	for _, inheritance := range inheritances {
		if inheritance != nil && inheritance.IsActive() && (inheritance.RoleID == roleID || inheritance.InheritedRoleID == roleID) {
			return perrors.WithCode(code.ErrRoleInUse, "role participates in an active inheritance")
		}
	}
	return nil
}
