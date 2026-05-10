package resource

import (
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// Resource 域对象资源目录（聚合根）
// V1：仅域对象类型，格式：<app>:<domain>:<type>:* 例如 scale:form:template:*
type Resource struct {
	ID          ResourceID
	Key         string   // 资源键，如 scale:form:template:*
	DisplayName string   // 显示名称
	AppName     string   // 应用名称
	Domain      string   // 业务域
	Type        string   // 对象类型
	Actions     []string // 允许的动作列表
	ScopeKinds  []scope.Kind
	Description string // 描述
}

// NewResource 创建新资源。
func NewResource(key string, actions []string, opts ...ResourceOption) (Resource, error) {
	resourceKey, err := NewKey(key)
	if err != nil {
		return Resource{}, err
	}
	normalizedActions, err := NormalizeActions(actions)
	if err != nil {
		return Resource{}, err
	}
	r := Resource{
		Key:     resourceKey.String(),
		Actions: normalizedActions,
	}
	for _, opt := range opts {
		opt(&r)
	}
	r.AppName = strings.TrimSpace(r.AppName)
	r.Domain = strings.TrimSpace(r.Domain)
	r.Type = strings.TrimSpace(r.Type)
	if r.AppName == "" {
		r.AppName = resourceKey.App()
	}
	if r.Domain == "" {
		r.Domain = resourceKey.Domain()
	}
	if r.Type == "" {
		r.Type = resourceKey.Type()
	}
	if r.AppName != resourceKey.App() {
		return Resource{}, perrors.WithCode(code.ErrInvalidArgument, "resource app does not match key")
	}
	if r.Domain != resourceKey.Domain() || r.Type != resourceKey.Type() {
		return Resource{}, perrors.WithCode(code.ErrInvalidArgument, "resource domain/type does not match key")
	}
	scopeKinds, err := NormalizeAndValidateScopeKinds(r.ScopeKinds)
	if err != nil {
		return Resource{}, err
	}
	r.ScopeKinds = scopeKinds
	return r, nil
}

// ResourceOption 资源选项
type ResourceOption func(*Resource)

func WithID(id ResourceID) ResourceOption        { return func(r *Resource) { r.ID = id } }
func WithDisplayName(name string) ResourceOption { return func(r *Resource) { r.DisplayName = name } }
func WithAppName(app string) ResourceOption      { return func(r *Resource) { r.AppName = app } }
func WithDomain(domain string) ResourceOption    { return func(r *Resource) { r.Domain = domain } }
func WithType(typ string) ResourceOption         { return func(r *Resource) { r.Type = typ } }
func WithDescription(desc string) ResourceOption { return func(r *Resource) { r.Description = desc } }
func WithScopeKinds(kinds []scope.Kind) ResourceOption {
	return func(r *Resource) { r.ScopeKinds = append([]scope.Kind(nil), kinds...) }
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

func (r *Resource) AllowsScopeKind(kind scope.Kind) bool {
	allowed, err := NormalizeAndValidateScopeKinds(r.ScopeKinds)
	if err != nil {
		return false
	}
	for _, candidate := range allowed {
		if candidate == kind {
			return true
		}
	}
	return false
}

func (r *Resource) Supports(action string, objectScope scope.Scope) bool {
	return r.HasAction(action) && r.AllowsScopeKind(objectScope.Normalized().Kind)
}

// ChangeCatalog updates only future authorization-write validation metadata.
// Existing permission facts are not reconciled, removed, or reloaded by this method.
func (r *Resource) ChangeCatalog(actions []string, scopeKinds []scope.Kind) error {
	normalizedActions, err := NormalizeActions(actions)
	if err != nil {
		return err
	}
	normalizedScopeKinds, err := NormalizeAndValidateScopeKinds(scopeKinds)
	if err != nil {
		return err
	}
	r.Actions = normalizedActions
	r.ScopeKinds = normalizedScopeKinds
	return nil
}

func NormalizeActions(actions []string) ([]string, error) {
	seen := make(map[string]struct{}, len(actions))
	normalized := make([]string, 0, len(actions))
	for _, action := range actions {
		action = strings.TrimSpace(action)
		if action == "" {
			continue
		}
		if _, exists := seen[action]; exists {
			continue
		}
		seen[action] = struct{}{}
		normalized = append(normalized, action)
	}
	if len(normalized) == 0 {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "动作列表不能为空")
	}
	return normalized, nil
}

func NormalizeScopeKinds(kinds []scope.Kind) []scope.Kind {
	normalized, err := normalizeScopeKinds(kinds, false)
	if err != nil {
		return []scope.Kind{scope.KindAll}
	}
	return normalized
}

func NormalizeAndValidateScopeKinds(kinds []scope.Kind) ([]scope.Kind, error) {
	return normalizeScopeKinds(kinds, true)
}

func normalizeScopeKinds(kinds []scope.Kind, validate bool) ([]scope.Kind, error) {
	if len(kinds) == 0 {
		return []scope.Kind{scope.KindAll}, nil
	}
	seen := make(map[scope.Kind]struct{}, len(kinds))
	normalized := make([]scope.Kind, 0, len(kinds))
	for _, kind := range kinds {
		if kind == "" {
			continue
		}
		if validate {
			switch kind {
			case scope.KindAll, scope.KindOrigin:
			default:
				return nil, perrors.WithCode(code.ErrInvalidArgument, "unsupported scope kind: %s", kind)
			}
		}
		if _, ok := seen[kind]; ok {
			continue
		}
		seen[kind] = struct{}{}
		normalized = append(normalized, kind)
	}
	if len(normalized) == 0 {
		return []scope.Kind{scope.KindAll}, nil
	}
	return normalized, nil
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
