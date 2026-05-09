package loginidentity

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type Repository interface {
	Create(ctx context.Context, identity *LoginIdentity) error
	GetByID(ctx context.Context, id meta.ID) (*LoginIdentity, error)
	GetByProviderKey(ctx context.Context, provider Provider, realm, identifier string) (*LoginIdentity, error)
	GetByGlobalIdentifier(ctx context.Context, provider Provider, globalIdentifier string) (*LoginIdentity, error)
	ListByUserID(ctx context.Context, userID meta.ID) ([]*LoginIdentity, error)
	UpdateStatus(ctx context.Context, id meta.ID, status Status) error
}
