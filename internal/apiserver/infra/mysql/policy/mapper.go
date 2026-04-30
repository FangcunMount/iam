package policy

import (
	"github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// Mapper PolicyVersion BO 和 PO 转换器
type Mapper struct{}

// NewMapper 创建 Mapper
func NewMapper() *Mapper {
	return &Mapper{}
}

// ToBO 将 PO 转换为 BO
func (m *Mapper) ToBO(po *PolicyVersionPO) *policy.PolicyVersion {
	if po == nil {
		return nil
	}

	pv := &policy.PolicyVersion{
		ID:        policy.PolicyVersionID(po.ID),
		TenantID:  po.TenantID,
		Version:   po.PolicyVersion,
		ChangedBy: po.ChangedBy,
		Reason:    po.Reason,
	}

	return pv
}

// ToPO 将 BO 转换为 PO
func (m *Mapper) ToPO(bo *policy.PolicyVersion) *PolicyVersionPO {
	if bo == nil {
		return nil
	}

	po := &PolicyVersionPO{
		TenantID:      bo.TenantID,
		PolicyVersion: bo.Version,
		ChangedBy:     bo.ChangedBy,
		Reason:        bo.Reason,
	}
	id := meta.FromUint64(bo.ID.Uint64()) // 来自业务对象，必定有效
	po.ID = id

	return po
}

// ToBOList 将 PO 列表转换为 BO 列表
func (m *Mapper) ToBOList(pos []*PolicyVersionPO) []*policy.PolicyVersion {
	if len(pos) == 0 {
		return nil
	}

	bos := make([]*policy.PolicyVersion, 0, len(pos))
	for _, po := range pos {
		if bo := m.ToBO(po); bo != nil {
			bos = append(bos, bo)
		}
	}

	return bos
}
