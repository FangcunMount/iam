package resource

import (
	"encoding/json"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Mapper Resource BO 和 PO 转换器
type Mapper struct{}

// NewMapper 创建 Mapper
func NewMapper() *Mapper {
	return &Mapper{}
}

// ToBO 将 PO 转换为 BO
func (m *Mapper) ToBO(po *ResourcePO) (*resource.Resource, error) {
	if po == nil {
		return nil, nil
	}

	// 解析 Actions JSON
	actions, err := m.parseActions(po.Actions)
	if err != nil {
		return nil, err
	}
	scopeKinds, err := m.parseScopeKinds(po.ScopeKinds)
	if err != nil {
		return nil, err
	}

	r, err := resource.NewResource(
		po.Key,
		actions,
		resource.WithID(resource.NewResourceID(po.ID.Uint64())),
		resource.WithDisplayName(po.DisplayName),
		resource.WithAppName(po.AppName),
		resource.WithDomain(po.Domain),
		resource.WithType(po.Type),
		resource.WithScopeKinds(scopeKinds),
		resource.WithDescription(po.Description),
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ToPO 将 BO 转换为 PO
func (m *Mapper) ToPO(bo *resource.Resource) *ResourcePO {
	if bo == nil {
		return nil
	}

	// 序列化 Actions 为 JSON
	actionsJSON, _ := m.serializeActions(bo.ActionStrings())
	scopeKindsJSON, _ := m.serializeScopeKinds(bo.ScopeKinds)

	po := &ResourcePO{
		Key:         bo.KeyString(),
		DisplayName: bo.DisplayName,
		AppName:     bo.AppName,
		Domain:      bo.Domain,
		Type:        bo.Type,
		Actions:     actionsJSON,
		ScopeKinds:  scopeKindsJSON,
		Description: bo.Description,
	}
	id := meta.FromUint64(bo.ID.Uint64()) // 来自业务对象，必定有效
	po.ID = id

	return po
}

// ToBOList 将 PO 列表转换为 BO 列表
func (m *Mapper) ToBOList(pos []*ResourcePO) ([]*resource.Resource, error) {
	if len(pos) == 0 {
		return nil, nil
	}

	bos := make([]*resource.Resource, 0, len(pos))
	for _, po := range pos {
		bo, err := m.ToBO(po)
		if err != nil {
			return nil, err
		}
		if bo != nil {
			bos = append(bos, bo)
		}
	}

	return bos, nil
}

// serializeActions 序列化动作列表为 JSON
func (m *Mapper) serializeActions(actions []string) (string, error) {
	if len(actions) == 0 {
		return "[]", nil
	}

	data, err := json.Marshal(actions)
	if err != nil {
		return "[]", err
	}
	return string(data), nil
}

// parseActions 解析 JSON 为动作列表
func (m *Mapper) parseActions(jsonStr string) ([]string, error) {
	if jsonStr == "" || jsonStr == "[]" {
		return []string{}, nil
	}

	var actions []string
	if err := json.Unmarshal([]byte(jsonStr), &actions); err != nil {
		return []string{}, err
	}
	return actions, nil
}

func (m *Mapper) serializeScopeKinds(kinds []scope.Kind) (string, error) {
	normalized, err := resource.NormalizeAndValidateScopeKinds(kinds)
	if err != nil {
		return `["all"]`, err
	}
	values := make([]string, 0, len(normalized))
	for _, kind := range normalized {
		values = append(values, string(kind))
	}
	data, err := json.Marshal(values)
	if err != nil {
		return `["all"]`, err
	}
	return string(data), nil
}

func (m *Mapper) parseScopeKinds(jsonStr string) ([]scope.Kind, error) {
	if jsonStr == "" || jsonStr == "[]" {
		return resource.NormalizeScopeKinds(nil), nil
	}
	var values []string
	if err := json.Unmarshal([]byte(jsonStr), &values); err != nil {
		return resource.NormalizeScopeKinds(nil), err
	}
	kinds := make([]scope.Kind, 0, len(values))
	for _, value := range values {
		kinds = append(kinds, scope.Kind(value))
	}
	return kinds, nil
}
