package permissiongrant

import (
	"context"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

type Repository interface {
	Create(ctx context.Context, grant *Grant) error
	Revoke(ctx context.Context, id meta.ID) error
	FindByID(ctx context.Context, id meta.ID) (*Grant, error)
	ListByRole(ctx context.Context, roleID meta.ID, tenantID string) ([]*Grant, error)
	ListActiveByResource(ctx context.Context, resourceID resource.ResourceID) ([]*Grant, error)
	ListActiveByTenant(ctx context.Context, tenantID string) ([]*Grant, error)
}
