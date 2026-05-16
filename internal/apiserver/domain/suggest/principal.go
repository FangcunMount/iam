package suggest

// OperatingPrincipal 是 suggest 所需的最小 operating 后台身份视图。
type OperatingPrincipal struct {
	OperatorID   int64
	TenantID     int64
	TenantDomain string
	OrgIDs       []int64
	RoleCodes    []string
	IsSuperAdmin bool
}
