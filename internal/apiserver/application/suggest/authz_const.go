package suggest

// Casbin 资源与动作（与 bootstrap 策略对齐；search_byMobile 可逐步在策略中授予）。
const (
	ResourceIAMProfileCollection = "iam:identity:collection:profiles"
	ActionSearch                 = "search"
	ActionSearchByMobile         = "search_by_mobile"
)
