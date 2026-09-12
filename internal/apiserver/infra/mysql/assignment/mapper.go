package assignment

import (
	"encoding/json"
	"fmt"
	assignmentDomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/assignment"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"math"
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

	opts := []assignmentDomain.Option{assignmentDomain.WithID(assignmentDomain.AssignmentID(po.ID)), assignmentDomain.WithGrantedBy(po.GrantedBy)}
	if po.OrgID != 0 || po.ScopeKind != "" || po.ScopeStoreIDs != nil {
		if po.OrgID > math.MaxInt64 {
			return nil, fmt.Errorf("invalid assignment scope company")
		}
		var values []string
		if po.ScopeStoreIDs != nil {
			if err := json.Unmarshal([]byte(*po.ScopeStoreIDs), &values); err != nil {
				return nil, fmt.Errorf("invalid assignment store IDs: %w", err)
			}
			if values == nil {
				return nil, fmt.Errorf("assignment store IDs must be an array")
			}
		}
		ids := make([]meta.ID, 0, len(values))
		for _, value := range values {
			id, err := meta.ParseID(value)
			if err != nil || id <= 0 {
				return nil, fmt.Errorf("invalid assignment store ID")
			}
			ids = append(ids, id)
		}
		value, err := scope.New(meta.ID(po.OrgID), scope.Kind(po.ScopeKind), ids)
		if err != nil {
			return nil, err
		}
		opts = append(opts, assignmentDomain.WithScope(value))
	}
	a, err := assignmentDomain.NewAssignment(
		assignmentDomain.SubjectType(po.SubjectType),
		meta.MustFromUint64(parseStoredID(po.SubjectID)),
		meta.FromUint64(po.RoleID),

		opts...,
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

		GrantedBy: bo.GrantedBy,
	}
	id := meta.FromUint64(bo.ID.Uint64()) // 来自业务对象，必定有效
	if value, ok := bo.Scope(); ok {
		po.OrgID = value.OrgID().Uint64()
		po.ScopeKind = string(value.Kind())
		ids := make([]string, 0, len(value.StoreIDs()))
		for _, storeID := range value.StoreIDs() {
			ids = append(ids, storeID.String())
		}
		encoded, _ := json.Marshal(ids) // A string slice is always JSON encodable.
		payload := string(encoded)
		po.ScopeStoreIDs = &payload
	}
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
