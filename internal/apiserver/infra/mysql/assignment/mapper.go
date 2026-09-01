package assignment

import (
	assignmentDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Mapper 负责 Assignment 领域对象和持久化对象之间的转换。
type Mapper struct{}

// NewMapper 创建 Mapper
func NewMapper() *Mapper {
	return &Mapper{}
}

// ToBO 将 PO 转换为 BO
func (m *Mapper) ToBO(po *AssignmentPO) (*assignmentDomain.Assignment, error) {
	if po == nil {
		return nil, nil
	}

	a, err := assignmentDomain.NewAssignment(
		assignmentDomain.SubjectType(po.SubjectType),
		meta.MustFromUint64(parseStoredID(po.SubjectID)),
		meta.FromUint64(po.RoleID),
		po.TenantID,
		assignmentDomain.WithID(assignmentDomain.AssignmentID(po.ID)),
		assignmentDomain.WithGrantedBy(po.GrantedBy),
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ToPO 将 BO 转换为 PO
func (m *Mapper) ToPO(bo *assignmentDomain.Assignment) *AssignmentPO {
	if bo == nil {
		return nil
	}

	po := &AssignmentPO{
		SubjectType: bo.SubjectTypeString(),
		SubjectID:   bo.SubjectID.String(),
		RoleID:      bo.RoleID.Uint64(),
		TenantID:    bo.TenantIDString(),
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
func (m *Mapper) ToBOList(pos []*AssignmentPO) ([]*assignmentDomain.Assignment, error) {
	if len(pos) == 0 {
		return nil, nil
	}

	bos := make([]*assignmentDomain.Assignment, 0, len(pos))
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
