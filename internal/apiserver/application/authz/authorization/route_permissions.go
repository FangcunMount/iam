package authorization

const (
	ResourceRoles            = "iam:authz:collection:roles"
	ResourceAssignments      = "iam:authz:collection:assignments"
	ResourcePermissionGrants = "iam:authz:collection:permission_grants"
	ResourceRoleInheritances = "iam:authz:collection:role_inheritances"
	ResourceResources        = "iam:authz:collection:resources"
	ResourceSessions         = "iam:authn:collection:sessions"
	ResourceJWKS             = "iam:authn:collection:jwks"
	ResourceWechatApps       = "iam:idp:collection:wechat_apps"
	ResourceCacheGovernance  = "iam:ops:collection:cache_governance"

	ActionCreate                = "create"
	ActionRead                  = "read"
	ActionUpdate                = "update"
	ActionDelete                = "delete"
	ActionList                  = "list"
	ActionGrant                 = "grant"
	ActionRevoke                = "revoke"
	ActionValidateAction        = "validate_action"
	ActionRevokeByLoginIdentity = "revoke_by_login_identity"
	ActionRevokeByUser          = "revoke_by_user"
)
