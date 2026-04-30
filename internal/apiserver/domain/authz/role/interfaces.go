// Package role 角色领域包
package role

import "context"

// Validator 角色验证器接口。
// 封装角色相关的验证规则
type Validator interface {
	ValidateCreateParameters(name, displayName, tenantID string) error
	// CheckNameUnique 检查名称唯一性
	CheckNameUnique(ctx context.Context, tenantID, name string) error
}
