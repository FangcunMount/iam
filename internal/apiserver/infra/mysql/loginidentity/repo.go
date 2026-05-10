package loginidentity

import (
	"context"
	"fmt"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authn "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"gorm.io/gorm"
)

type Repository struct {
	mysql.BaseRepository[*PO]
	db     *gorm.DB
	mapper *Mapper
}

func NewRepository(db *gorm.DB) *Repository {
	base := mysql.NewBaseRepository[*PO](db)
	base.SetErrorTranslator(mysql.NewDuplicateToTranslator(func(e error) error {
		return perrors.WithCode(code.ErrLoginIdentityExists, "login identity already exists")
	}))
	return &Repository{
		BaseRepository: base,
		db:             db,
		mapper:         NewMapper(),
	}
}

func (r *Repository) Create(ctx context.Context, identity *domain.LoginIdentity) error {
	po := r.mapper.ToPO(identity)
	if err := r.CreateAndSync(ctx, po, func(updated *PO) {
		identity.ID = updated.ID
		identity.CreatedAt = updated.CreatedAt
		identity.UpdatedAt = updated.UpdatedAt
	}); err != nil {
		return fmt.Errorf("failed to create login identity: %w", err)
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id meta.ID) (*domain.LoginIdentity, error) {
	var po PO
	if err := r.WithContext(ctx).Where("id = ?", id).First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get login identity by id: %w", err)
	}
	return r.mapper.ToDO(&po), nil
}

func (r *Repository) GetByProviderKey(ctx context.Context, provider domain.Provider, realm, identifier string) (*domain.LoginIdentity, error) {
	var po PO
	if err := r.WithContext(ctx).
		Where("provider = ? AND realm = ? AND identifier = ?", string(provider), realm, identifier).
		First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get login identity by provider key: %w", err)
	}
	return r.mapper.ToDO(&po), nil
}

func (r *Repository) GetByGlobalIdentifier(ctx context.Context, provider domain.Provider, globalIdentifier string) (*domain.LoginIdentity, error) {
	globalIdentifier = strings.TrimSpace(globalIdentifier)
	if globalIdentifier == "" {
		return nil, nil
	}
	var po PO
	if err := r.WithContext(ctx).
		Where("provider = ? AND global_identifier = ?", string(provider), globalIdentifier).
		First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get login identity by global identifier: %w", err)
	}
	return r.mapper.ToDO(&po), nil
}

func (r *Repository) ListByUserID(ctx context.Context, userID meta.ID) ([]*domain.LoginIdentity, error) {
	var pos []PO
	if err := r.WithContext(ctx).Where("user_id = ?", userID).Find(&pos).Error; err != nil {
		return nil, fmt.Errorf("failed to list login identities by user: %w", err)
	}
	result := make([]*domain.LoginIdentity, 0, len(pos))
	for i := range pos {
		result = append(result, r.mapper.ToDO(&pos[i]))
	}
	return result, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id meta.ID, status domain.Status) error {
	result := r.WithContext(ctx).
		Model(&PO{}).
		Where("id = ?", id).
		Update("status", string(status))
	if result.Error != nil {
		return fmt.Errorf("failed to update login identity status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.NotFoundError()
	}
	return nil
}

func (r *Repository) FindUsernameIdentity(ctx context.Context, tenantID meta.ID, username string) (*authn.LoginIdentityLookup, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, nil
	}
	realm := domain.RealmDefault
	if !tenantID.IsZero() {
		realm = tenantID.String()
	}
	return r.FindLoginIdentityByProviderKey(ctx, domain.ProviderUsername, realm, username)
}

func (r *Repository) FindLoginIdentityByProviderKey(ctx context.Context, provider domain.Provider, realm, identifier string) (*authn.LoginIdentityLookup, error) {
	realm = strings.TrimSpace(realm)
	identifier = strings.TrimSpace(identifier)
	if realm == "" || identifier == "" {
		return nil, nil
	}
	identity, err := r.GetByProviderKey(ctx, provider, realm, identifier)
	if err != nil {
		return nil, err
	}
	if identity != nil {
		return toLookup(identity), nil
	}
	return nil, nil
}

func (r *Repository) FindLoginIdentityByGlobalIdentifier(ctx context.Context, provider domain.Provider, globalIdentifier string) (*authn.LoginIdentityLookup, error) {
	identity, err := r.GetByGlobalIdentifier(ctx, provider, globalIdentifier)
	if err != nil {
		return nil, err
	}
	if identity != nil {
		return toLookup(identity), nil
	}
	return nil, nil
}

func (r *Repository) IsLoginIdentityActive(ctx context.Context, loginIdentityID meta.ID) (bool, error) {
	identity, err := r.GetByID(ctx, loginIdentityID)
	if err != nil {
		return false, err
	}
	if identity != nil {
		return identity.Status == domain.StatusActive, nil
	}
	return false, nil
}

func toLookup(identity *domain.LoginIdentity) *authn.LoginIdentityLookup {
	if identity == nil {
		return nil
	}
	return &authn.LoginIdentityLookup{
		LoginIdentityID:  identity.ID,
		UserID:           identity.UserID,
		Provider:         identity.Provider,
		Realm:            identity.Realm,
		Identifier:       identity.Identifier,
		GlobalIdentifier: identity.GlobalIdentifier,
		Status:           identity.Status,
	}
}
