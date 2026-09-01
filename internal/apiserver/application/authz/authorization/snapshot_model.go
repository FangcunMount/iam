package authorization

type AuthorizationMode string

const (
	ModeUnconditional       AuthorizationMode = "UNCONDITIONAL"
	ModeObjectCheckRequired AuthorizationMode = "OBJECT_CHECK_REQUIRED"
)

type PermissionEntry struct {
	Resource string
	Action   string
	Mode     AuthorizationMode
}

type SubjectSnapshot struct {
	DirectRoles    []string
	EffectiveRoles []string
	Permissions    []PermissionEntry
	PolicyVersion  int64
}
