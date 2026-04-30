package resource

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
	authzDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// Resource 域对象资源目录（聚合根）
// V1：仅域对象类型，格式：<app>:<domain>:<type>:* 例如 scale:form:*
type Resource struct {
	ID          ResourceID
	Key         string   // 资源键，如 scale:form:*
	DisplayName string   // 显示名称
	AppName     string   // 应用名称
	Domain      string   // 业务域
	Type        string   // 对象类型
	Actions     []string // 允许的动作列表
	ScopeKinds  []authzDomain.ScopeKind
	Description string // 描述
}

// NewResource 创建新资源
func NewResource(key string, actions []string, opts ...ResourceOption) Resource {
	r := Resource{
		Key:     key,
		Actions: actions,
	}
	for _, opt := range opts {
		opt(&r)
	}
	return r
}

// ResourceOption 资源选项
type ResourceOption func(*Resource)

func WithID(id ResourceID) ResourceOption        { return func(r *Resource) { r.ID = id } }
func WithDisplayName(name string) ResourceOption { return func(r *Resource) { r.DisplayName = name } }
func WithAppName(app string) ResourceOption      { return func(r *Resource) { r.AppName = app } }
func WithDomain(domain string) ResourceOption    { return func(r *Resource) { r.Domain = domain } }
func WithType(typ string) ResourceOption         { return func(r *Resource) { r.Type = typ } }
func WithDescription(desc string) ResourceOption { return func(r *Resource) { r.Description = desc } }
func WithScopeKinds(kinds []authzDomain.ScopeKind) ResourceOption {
	return func(r *Resource) { r.ScopeKinds = NormalizeScopeKinds(kinds) }
}

// HasAction 检查资源是否包含指定动作
func (r *Resource) HasAction(action string) bool {
	for _, a := range r.Actions {
		if a == action {
			return true
		}
	}
	return false
}

func (r *Resource) AllowsScopeKind(kind authzDomain.ScopeKind) bool {
	allowed := NormalizeScopeKinds(r.ScopeKinds)
	for _, candidate := range allowed {
		if candidate == kind {
			return true
		}
	}
	return false
}

func (r *Resource) Supports(action string, scope authzDomain.Scope) bool {
	return r.HasAction(action) && r.AllowsScopeKind(scope.Normalized().Kind)
}

func (r *Resource) ChangeCatalog(actions []string, scopeKinds []authzDomain.ScopeKind) error {
	if len(actions) == 0 {
		return perrors.WithCode(code.ErrInvalidArgument, "动作列表不能为空")
	}
	r.Actions = actions
	r.ScopeKinds = NormalizeScopeKinds(scopeKinds)
	return nil
}

func NormalizeScopeKinds(kinds []authzDomain.ScopeKind) []authzDomain.ScopeKind {
	if len(kinds) == 0 {
		return []authzDomain.ScopeKind{authzDomain.ScopeKindAll}
	}
	seen := make(map[authzDomain.ScopeKind]struct{}, len(kinds))
	normalized := make([]authzDomain.ScopeKind, 0, len(kinds))
	for _, kind := range kinds {
		if kind == "" {
			continue
		}
		if _, ok := seen[kind]; ok {
			continue
		}
		seen[kind] = struct{}{}
		normalized = append(normalized, kind)
	}
	if len(normalized) == 0 {
		return []authzDomain.ScopeKind{authzDomain.ScopeKindAll}
	}
	return normalized
}

// ResourceID 资源ID值对象
type ResourceID idutil.ID

func NewResourceID(value uint64) ResourceID {
	return ResourceID(idutil.NewID(value)) // 从 uint64 构造
}

func (id ResourceID) Uint64() uint64 {
	return idutil.ID(id).Uint64()
}

func (id ResourceID) String() string {
	return idutil.ID(id).String()
}
