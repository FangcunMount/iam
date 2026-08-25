package suggest

// OperatingPrincipal 是 suggest 所需的最小 operating 后台身份视图。
type OperatingPrincipal struct {
	OperatorID int64
	// TenantDomain IAM 授权域，如 fangcun / platform。
	TenantDomain string
	// OrgID 业务组织可见范围，来自 JWT org_id 透传；非 IAM 核心身份字段。
	OrgID int64
	// OrgIDs 可选的多组织范围（业务侧提供）。
	OrgIDs       []int64
	RoleCodes    []string
	IsSuperAdmin bool
}
