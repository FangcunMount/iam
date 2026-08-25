package roleinheritance

import (
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

type Mapper struct{}

func (Mapper) ToPO(inheritance *domain.Inheritance) *InheritancePO {
	if inheritance == nil {
		return nil
	}
	po := &InheritancePO{
		TenantID:        inheritance.TenantIDString(),
		RoleID:          inheritance.RoleID.Uint64(),
		InheritedRoleID: inheritance.InheritedRoleID.Uint64(),
		GrantedBy:       inheritance.GrantedBy,
		GrantedAt:       inheritance.GrantedAt,
		RevokedAt:       inheritance.RevokedAt,
	}
	po.ID = inheritance.ID
	po.Version = inheritance.Version
	return po
}

func (Mapper) ToBO(po *InheritancePO) (*domain.Inheritance, error) {
	if po == nil {
		return nil, nil
	}
	inheritance, err := domain.Restore(
		meta.FromUint64(po.RoleID),
		meta.FromUint64(po.InheritedRoleID),
		po.TenantID,
		po.GrantedBy,
		domain.RestoreOptions{
			ID:        po.ID,
			GrantedAt: po.GrantedAt,
			RevokedAt: po.RevokedAt,
			Version:   po.Version,
		},
	)
	if err != nil {
		return nil, err
	}
	return &inheritance, nil
}
