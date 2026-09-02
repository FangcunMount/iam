package visibility

// Principal 是 suggest 所需的最小 operating 后台身份视图。
type Principal struct {
	OperatorID   int64
	TenantDomain string
	OrgID        int64
	OrgIDs       []int64
}
