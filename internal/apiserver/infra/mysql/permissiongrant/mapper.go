package permissiongrant

import (
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

type Mapper struct{}

func (Mapper) ToPO(grant *domain.Grant) (*GrantPO, error) {
	if grant == nil {
		return nil, nil
	}
	encoded, err := grant.CanonicalConstraintJSON()
	if err != nil {
		return nil, err
	}
	po := &GrantPO{
		TenantID:        grant.TenantIDString(),
		RoleID:          grant.RoleID.Uint64(),
		ResourcePattern: grant.ResourcePatternString(),
		Action:          grant.ActionString(),
		ConstraintSet:   string(encoded),
		GrantKey:        grant.GrantKey,
		GrantedBy:       grant.GrantedBy,
		GrantedAt:       grant.GrantedAt,
		RevokedAt:       grant.RevokedAt,
	}
	if grant.ResourceID.Uint64() != 0 {
		resourceID := grant.ResourceID.Uint64()
		po.ResourceID = &resourceID
	}
	po.ID = grant.ID
	po.Version = grant.Version
	return po, nil
}

func (Mapper) ToBO(po *GrantPO) (*domain.Grant, error) {
	if po == nil {
		return nil, nil
	}
	constraints, err := constraint.ParseJSON([]byte(po.ConstraintSet))
	if err != nil {
		return nil, err
	}
	resourceID := resource.NewResourceID(0)
	if po.ResourceID != nil {
		resourceID = resource.NewResourceID(*po.ResourceID)
	}
	grant, err := domain.Restore(
		meta.FromUint64(po.RoleID),
		po.TenantID,
		resourceID,
		po.ResourcePattern,
		po.Action,
		constraints,
		po.GrantedBy,
		domain.RestoreOptions{
			ID:        po.ID,
			GrantKey:  po.GrantKey,
			GrantedAt: po.GrantedAt,
			RevokedAt: po.RevokedAt,
			Version:   po.Version,
		},
	)
	if err != nil {
		return nil, err
	}
	return &grant, nil
}
