package queryprofile

// AuthZ v3 资源与动作（与 bootstrap PermissionGrant 对齐）。
const (
	ResourceIAMProfileCollection = "iam:identity:collection:profiles"
	ActionList                   = "list"
	ActionSearch                 = "search"
	ActionSearchByMobile         = "search_by_mobile"
)
