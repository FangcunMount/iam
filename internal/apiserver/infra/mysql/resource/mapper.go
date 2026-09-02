package resource

import (
	"encoding/json"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
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
	attributeSchema, err := m.parseAttributeSchema(po.AttributeSchema)
	if err != nil {
		return nil, err
	}

	r, err := resource.RestoreResource(
		po.Key,
		actions,
		resource.WithID(resource.NewResourceID(po.ID.Uint64())),
		resource.WithDisplayName(po.DisplayName),
		resource.WithAppName(po.AppName),
		resource.WithDomain(po.Domain),
		resource.WithType(po.Type),
		resource.WithAttributeSchema(attributeSchema),
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
	attributeSchemaJSON, _ := m.serializeAttributeSchema(bo.AttributeSchema)

	po := &ResourcePO{
		Key:             bo.KeyString(),
		DisplayName:     bo.DisplayName,
		AppName:         bo.AppName,
		Domain:          bo.Domain,
		Type:            bo.Type,
		Actions:         actionsJSON,
		AttributeSchema: attributeSchemaJSON,
		Description:     bo.Description,
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

func (m *Mapper) serializeAttributeSchema(schema attribute.Schema) (string, error) {
	normalized, err := schema.Normalize()
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (m *Mapper) parseAttributeSchema(jsonStr string) (attribute.Schema, error) {
	if jsonStr == "" || jsonStr == "null" {
		return attribute.EmptySchema(), nil
	}
	var schema attribute.Schema
	if err := json.Unmarshal([]byte(jsonStr), &schema); err != nil {
		return attribute.Schema{}, err
	}
	return schema.Normalize()
}
