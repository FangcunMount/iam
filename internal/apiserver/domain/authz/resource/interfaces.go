// Package resource 资源领域包
package resource

import (
	"context"

	authzDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz"
)

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

// Validator 资源验证器接口
// 封装资源相关的验证规则
type Validator interface {
	ValidateCreateParameters(key string, displayName string, appName string, domain string, resourceType string, actions []string) error
	ValidateUpdateParameters(actions []string) error
	ValidateScopeKinds(kinds []authzDomain.ScopeKind) error
	// CheckKeyUnique 检查键唯一性
	CheckKeyUnique(ctx context.Context, key string) error
}
