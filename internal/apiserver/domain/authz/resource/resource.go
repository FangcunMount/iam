package resource

import (
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/util/idutil"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// Resource 域对象资源目录（聚合根）
// V1：仅域对象类型，格式：<app>:<domain>:<type>:* 例如 scale:form:template:*
type Resource struct {
	ID              ResourceID
	Key             Key      // 资源键，如 scale:form:template:*
	DisplayName     string   // 显示名称
	AppName         string   // 应用名称
	Domain          string   // 业务域
	Type            string   // 对象类型
	Actions         []Action // 允许的动作列表
	AttributeSchema attribute.Schema
	Description     string // 描述
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
		Key:     resourceKey,
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
	attributeSchema, err := r.AttributeSchema.Normalize()
	if err != nil {
		return Resource{}, err
	}
	r.AttributeSchema = attributeSchema
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
func WithAttributeSchema(schema attribute.Schema) ResourceOption {
	return func(r *Resource) { r.AttributeSchema = schema }
}

func (r Resource) KeyString() string {
	return r.Key.String()
}

func (r Resource) ActionStrings() []string {
	if len(r.Actions) == 0 {
		return nil
	}
	actions := make([]string, 0, len(r.Actions))
	for _, action := range r.Actions {
		actions = append(actions, action.String())
	}
	return actions
}

// HasAction 检查资源是否包含指定动作
func (r *Resource) HasAction(action string) bool {
	target, err := NewAction(action)
	if err != nil {
		return false
	}
	for _, a := range r.Actions {
		if a == target {
			return true
		}
	}
	return false
}

func (r *Resource) ChangeAttributeSchema(schema attribute.Schema) error {
	normalized, err := schema.Normalize()
	if err != nil {
		return err
	}
	r.AttributeSchema = normalized
	return nil
}

// ChangeCatalog updates only future authorization-write validation metadata.
// Existing permission facts are not reconciled, removed, or reloaded by this method.
func (r *Resource) ChangeCatalog(actions []string) error {
	normalizedActions, err := NormalizeActions(actions)
	if err != nil {
		return err
	}
	r.Actions = normalizedActions
	return nil
}

func NormalizeActions(actions []string) ([]Action, error) {
	seen := make(map[string]struct{}, len(actions))
	normalized := make([]Action, 0, len(actions))
	for _, action := range actions {
		actionValue, err := NewAction(action)
		if err != nil {
			if strings.TrimSpace(action) == "" {
				continue
			}
			return nil, err
		}
		actionKey := actionValue.String()
		if _, exists := seen[actionKey]; exists {
			continue
		}
		seen[actionKey] = struct{}{}
		normalized = append(normalized, actionValue)
	}
	if len(normalized) == 0 {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "动作列表不能为空")
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
