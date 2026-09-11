package authorization

type AuthorizationMode string

const (
	ModeUnconditional AuthorizationMode = "UNCONDITIONAL"
)

type PermissionEntry struct {
	Resource string
	Action   string
	Mode     AuthorizationMode
}

type AssignmentRoleFact struct {
	RoleID, RoleName, ManagementProtection string
}

type SubjectSnapshot struct {
	AssignmentFacts         []AssignmentRoleFact
	AssignmentFactsComplete bool
	DirectRoles             []string
	EffectiveRoles          []string
	Permissions             []PermissionEntry
	PolicyVersion           int64
}
