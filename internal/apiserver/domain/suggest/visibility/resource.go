package visibility

// Resource 表达 scope 过滤所需的最小档案维度。
type Resource struct {
	ProfileID        int64
	OrgID            int64
	OwnerOperatorIDs []int64
}
