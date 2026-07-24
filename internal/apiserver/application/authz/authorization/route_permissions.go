package authorization

const (
	ResourceCheck       = "iam:authz:action:check"
	ResourceRoles       = "iam:authz:collection:roles"
	ResourceAssignments = "iam:authz:collection:assignments"
	ResourcePolicies    = "iam:authz:collection:policies"
	ResourceResources   = "iam:authz:collection:resources"

	ActionCheck          = "check"
	ActionCreate         = "create"
	ActionRead           = "read"
	ActionUpdate         = "update"
	ActionDelete         = "delete"
	ActionList           = "list"
	ActionGrant          = "grant"
	ActionRevoke         = "revoke"
	ActionWrite          = "write"
	ActionValidateAction = "validate_action"
)
