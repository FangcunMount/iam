package rolebinding

import (
	binding "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// Mapper Binding BO 和 PO 转换器
type Mapper struct{}

// NewMapper 创建 Mapper
func NewMapper() *Mapper {
	return &Mapper{}
}

// ToBO 将 PO 转换为 BO
func (m *Mapper) ToBO(po *BindingPO) (*binding.Binding, error) {
	if po == nil {
		return nil, nil
	}

	a, err := binding.NewBinding(
		binding.SubjectType(po.SubjectType),
		meta.MustFromUint64(parseStoredID(po.SubjectID)),
		meta.FromUint64(po.RoleID),
		po.TenantID,
		binding.WithID(binding.BindingID(po.ID)),
		binding.WithGrantedBy(po.GrantedBy),
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ToPO 将 BO 转换为 PO
func (m *Mapper) ToPO(bo *binding.Binding) *BindingPO {
	if bo == nil {
		return nil
	}

	po := &BindingPO{
		SubjectType: string(bo.SubjectType),
		SubjectID:   bo.SubjectID.String(),
		RoleID:      bo.RoleID.Uint64(),
		TenantID:    bo.TenantID,
		GrantedBy:   bo.GrantedBy,
	}
	id := meta.FromUint64(bo.ID.Uint64()) // 来自业务对象，必定有效
	po.ID = id

	return po
}

func parseStoredID(value string) uint64 {
	id, err := meta.ParseID(value)
	if err != nil {
		return 0
	}
	return id.Uint64()
}

// ToBOList 将 PO 列表转换为 BO 列表
func (m *Mapper) ToBOList(pos []*BindingPO) ([]*binding.Binding, error) {
	if len(pos) == 0 {
		return nil, nil
	}

	bos := make([]*binding.Binding, 0, len(pos))
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
