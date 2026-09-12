package authorization

import "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"

type AuthorizationMode string

const (
	ModeUnconditional AuthorizationMode = "UNCONDITIONAL"
)

type PermissionEntry struct {
	// Scopes are the union only of assignments granting this resource/action.
	// Empty means no configured business range, never unrestricted access.
	Scopes   []scope.Scope
	Resource string
	Action   string
	Mode     AuthorizationMode
}

type AssignmentRoleFact struct {
	RoleID, RoleName, ManagementProtection string
}

// AssignmentScopeFact preserves each assignment; nil Scope is explicitly unconfigured.
type AssignmentScopeFact struct {
	AssignmentID string
	Role         AssignmentRoleFact
	Scope        *scope.Scope
}

type SubjectSnapshot struct {
	AssignmentScopes        []AssignmentScopeFact
	AssignmentFacts         []AssignmentRoleFact
	AssignmentFactsComplete bool
	DirectRoles             []string
	EffectiveRoles          []string
	Permissions             []PermissionEntry
	PolicyVersion           int64
}
