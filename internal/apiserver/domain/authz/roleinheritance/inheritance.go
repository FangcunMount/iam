package roleinheritance

import (
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

type Inheritance struct {
	ID              meta.ID
	TenantID        tenant.ID
	RoleID          meta.ID
	InheritedRoleID meta.ID
	GrantedBy       string
	GrantedAt       time.Time
	RevokedAt       *time.Time
	Version         uint32
}

func New(roleID, inheritedRoleID meta.ID, tenantID, grantedBy string) (Inheritance, error) {
	if roleID.IsZero() || inheritedRoleID.IsZero() {
		return Inheritance{}, perrors.WithCode(code.ErrInvalidArgument, "role ids are required")
	}
	if roleID == inheritedRoleID {
		return Inheritance{}, perrors.WithCode(code.ErrInvalidArgument, "role cannot inherit itself")
	}
	tenantIDValue, err := tenant.NewID(tenantID)
	if err != nil {
		return Inheritance{}, err
	}
	grantedBy = strings.TrimSpace(grantedBy)
	if grantedBy == "" {
		return Inheritance{}, perrors.WithCode(code.ErrInvalidArgument, "granted by is required")
	}
	return Inheritance{
		TenantID:        tenantIDValue,
		RoleID:          roleID,
		InheritedRoleID: inheritedRoleID,
		GrantedBy:       grantedBy,
		Version:         1,
	}, nil
}

func (i Inheritance) TenantIDString() string { return i.TenantID.String() }
func (i Inheritance) IsActive() bool         { return i.RevokedAt == nil }

func (i *Inheritance) Revoke(at time.Time) error {
	if i == nil {
		return perrors.WithCode(code.ErrInvalidArgument, "role inheritance is required")
	}
	if i.RevokedAt != nil {
		return nil
	}
	if at.IsZero() {
		at = time.Now()
	}
	i.RevokedAt = &at
	return nil
}

type RestoreOptions struct {
	ID        meta.ID
	GrantedAt time.Time
	RevokedAt *time.Time
	Version   uint32
}

func Restore(roleID, inheritedRoleID meta.ID, tenantID, grantedBy string, options RestoreOptions) (Inheritance, error) {
	inheritance, err := New(roleID, inheritedRoleID, tenantID, grantedBy)
	if err != nil {
		return Inheritance{}, err
	}
	inheritance.ID = options.ID
	inheritance.GrantedAt = options.GrantedAt
	inheritance.RevokedAt = options.RevokedAt
	if options.Version > 0 {
		inheritance.Version = options.Version
	}
	return inheritance, nil
}
