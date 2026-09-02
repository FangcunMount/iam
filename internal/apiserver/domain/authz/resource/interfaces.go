// Package resource 资源领域包
package resource

// ResourceFilter describes repository-level resource filtering.
type ResourceFilter struct {
	// AppName 应用名称过滤
	AppName string

	// Domain 业务域过滤
	Domain string

	// Type 资源类型过滤
	Type string

	// Offset 分页偏移量
	Offset int

	// Limit 分页限制（每页数量）
	Limit int
}
