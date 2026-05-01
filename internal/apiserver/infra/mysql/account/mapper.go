package account

import (
	"encoding/json"

	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/account"
)

// Mapper 负责领域模型与持久化对象之间的转换。
type Mapper struct{}

// NewMapper 创建新的映射器实例。
func NewMapper() *Mapper {
	return &Mapper{}
}

// ==================== Account 映射 ====================

// ToAccountPO 将账号领域模型转换为持久化对象。
func (m *Mapper) ToAccountPO(acc *domain.Account) *AccountPO {
	if acc == nil {
		return nil
	}

	po := &AccountPO{
		UserID:         acc.UserID,
		Type:           string(acc.Type),
		ExternalID:     string(acc.ExternalID),
		ScopedTenantID: acc.ScopedTenantID,
		Profile:        mapToJSON(acc.Profile),
		Meta:           mapToJSON(acc.Meta),
		Status:         int8(acc.Status),
	}

	// 设置 ID（如果已存在）
	if acc.ID.Uint64() != 0 {
		po.ID = acc.ID
	}

	// 设置 AppID
	if acc.AppID != "" {
		appIDStr := string(acc.AppID)
		po.AppID = &appIDStr
	}

	// 设置 UniqueID
	if acc.UniqueID != "" {
		uniqueIDStr := string(acc.UniqueID)
		po.UniqueID = &uniqueIDStr
	}

	return po
}

// ToAccountDO 将持久化对象转换为账号领域模型。
func (m *Mapper) ToAccountDO(po *AccountPO) *domain.Account {
	if po == nil {
		return nil
	}

	acc := &domain.Account{
		ID:             po.ID,
		UserID:         po.UserID,
		Type:           domain.AccountType(po.Type),
		ExternalID:     domain.ExternalID(po.ExternalID),
		ScopedTenantID: po.ScopedTenantID,
		Profile:        jsonToMap(po.Profile),
		Meta:           jsonToMap(po.Meta),
		Status:         domain.AccountStatus(po.Status),
	}

	// 设置 AppID
	if po.AppID != nil {
		acc.AppID = domain.AppId(*po.AppID)
	}

	// 设置 UniqueID
	if po.UniqueID != nil {
		acc.UniqueID = domain.UnionID(*po.UniqueID)
	}

	return acc
}

// mapToJSON 将 map 转换为 JSON 字节数组。
func mapToJSON(m map[string]string) []byte {
	if len(m) == 0 {
		return nil
	}
	data, _ := json.Marshal(m)
	return data
}

// jsonToMap 将 JSON 字节数组转换为 map。
func jsonToMap(data []byte) map[string]string {
	if len(data) == 0 {
		return make(map[string]string)
	}
	var m map[string]string
	_ = json.Unmarshal(data, &m)
	if m == nil {
		return make(map[string]string)
	}
	return m
}
