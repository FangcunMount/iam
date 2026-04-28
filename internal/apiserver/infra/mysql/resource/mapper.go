package resource

import (
	"encoding/json"

	"github.com/FangcunMount/iam/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// Mapper Resource BO 和 PO 转换器
type Mapper struct{}

// NewMapper 创建 Mapper
func NewMapper() *Mapper {
	return &Mapper{}
}

// ToBO 将 PO 转换为 BO
func (m *Mapper) ToBO(po *ResourcePO) *resource.Resource {
	if po == nil {
		return nil
	}

	// 解析 Actions JSON
	actions, _ := m.parseActions(po.Actions)

	r := &resource.Resource{
		ID:          resource.NewResourceID(po.ID.Uint64()),
		Key:         po.Key,
		DisplayName: po.DisplayName,
		AppName:     po.AppName,
		Domain:      po.Domain,
		Type:        po.Type,
		Actions:     actions,
		Description: po.Description,
	}

	return r
}

// ToPO 将 BO 转换为 PO
func (m *Mapper) ToPO(bo *resource.Resource) *ResourcePO {
	if bo == nil {
		return nil
	}

	// 序列化 Actions 为 JSON
	actionsJSON, _ := m.serializeActions(bo.Actions)

	po := &ResourcePO{
		Key:         bo.Key,
		DisplayName: bo.DisplayName,
		AppName:     bo.AppName,
		Domain:      bo.Domain,
		Type:        bo.Type,
		Actions:     actionsJSON,
		Description: bo.Description,
	}
	id := meta.FromUint64(bo.ID.Uint64()) // 来自业务对象，必定有效
	po.ID = id

	return po
}

// ToBOList 将 PO 列表转换为 BO 列表
func (m *Mapper) ToBOList(pos []*ResourcePO) []*resource.Resource {
	if len(pos) == 0 {
		return nil
	}

	bos := make([]*resource.Resource, 0, len(pos))
	for _, po := range pos {
		if bo := m.ToBO(po); bo != nil {
			bos = append(bos, bo)
		}
	}

	return bos
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
