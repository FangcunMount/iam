package credential

import (
	"context"
	"fmt"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"gorm.io/gorm"
)

type Repository struct {
	mysql.BaseRepository[*V2PO]
	db     *gorm.DB
	mapper *Mapper
}

func NewRepository(db *gorm.DB) *Repository {
	base := mysql.NewBaseRepository[*V2PO](db)
	base.SetErrorTranslator(translateDuplicateCredentialError)
	return &Repository{
		BaseRepository: base,
		db:             db,
		mapper:         NewMapper(),
	}
}

func (r *Repository) Create(ctx context.Context, cred *domain.Credential) error {
	po := r.mapper.ToPO(cred)
	if err := r.WithContext(ctx).Create(po).Error; err != nil {
		return translateDuplicateCredentialError(err)
	}
	cred.ID = po.ID
	return nil
}

func translateDuplicateCredentialError(err error) error {
	return mysql.NewDuplicateToTranslator(func(error) error {
		return perrors.WithCode(code.ErrCredentialExists, "credential already exists")
	})(err)
}

func (r *Repository) UpdateMaterial(ctx context.Context, id meta.ID, material []byte, algo string) error {
	return r.updateCredential(ctx, id, map[string]interface{}{
		"material": material,
		"algo":     algo,
	}, "material")
}

func (r *Repository) UpdateStatus(ctx context.Context, id meta.ID, status domain.CredentialStatus) error {
	return r.updateCredential(ctx, id, map[string]interface{}{"status": status.String()}, "status")
}

func (r *Repository) UpdateFailedAttempts(ctx context.Context, id meta.ID, attempts int) error {
	return r.updateCredential(ctx, id, map[string]interface{}{"failed_attempts": attempts}, "failed_attempts")
}

func (r *Repository) UpdateLockedUntil(ctx context.Context, id meta.ID, lockedUntil *time.Time) error {
	return r.updateCredential(ctx, id, map[string]interface{}{"locked_until": lockedUntil}, "locked_until")
}

func (r *Repository) UpdateLastSuccessAt(ctx context.Context, id meta.ID, lastSuccessAt time.Time) error {
	return r.updateCredential(ctx, id, map[string]interface{}{"last_success_at": lastSuccessAt}, "last_success_at")
}

func (r *Repository) UpdateLastFailureAt(ctx context.Context, id meta.ID, lastFailureAt time.Time) error {
	return r.updateCredential(ctx, id, map[string]interface{}{"last_failure_at": lastFailureAt}, "last_failure_at")
}

func (r *Repository) UpdateExpiresAt(ctx context.Context, id meta.ID, expiresAt *time.Time) error {
	return fmt.Errorf("UpdateExpiresAt not implemented: expires_at field not defined in credential PO")
}

func (r *Repository) GetByID(ctx context.Context, id meta.ID) (*domain.Credential, error) {
	var po V2PO
	if err := r.WithContext(ctx).Where("id = ?", id.Uint64()).First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get credential by id: %w", err)
	}
	return r.mapper.ToDO(&po), nil
}

func (r *Repository) GetByLoginIdentityIDAndType(ctx context.Context, loginIdentityID meta.ID, credType domain.CredentialType) (*domain.Credential, error) {
	var po V2PO
	if err := r.WithContext(ctx).
		Where("login_identity_id = ? AND type = ?", loginIdentityID.Uint64(), string(credType)).
		First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get credential by login identity/type: %w", err)
	}
	return r.mapper.ToDO(&po), nil
}

func (r *Repository) ListByLoginIdentityID(ctx context.Context, loginIdentityID meta.ID) ([]*domain.Credential, error) {
	var pos []V2PO
	if err := r.WithContext(ctx).
		Where("login_identity_id = ?", loginIdentityID.Uint64()).
		Find(&pos).Error; err != nil {
		return nil, fmt.Errorf("failed to list credential by login identity: %w", err)
	}
	result := make([]*domain.Credential, 0, len(pos))
	for i := range pos {
		result = append(result, r.mapper.ToDO(&pos[i]))
	}
	return result, nil
}

func (r *Repository) Delete(ctx context.Context, id meta.ID) error {
	result := r.WithContext(ctx).
		Where("id = ?", id.Uint64()).
		Delete(&V2PO{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete credential: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return credentialNotFoundError()
	}
	return nil
}

func (r *Repository) FindPasswordCredentialByLoginIdentity(ctx context.Context, loginIdentityID meta.ID) (credentialID meta.ID, passwordHash string, err error) {
	var po V2PO
	if err := r.WithContext(ctx).
		Select("id", "material").
		Where("login_identity_id = ? AND type = ?", loginIdentityID, string(domain.CredPassword)).
		First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return meta.ZeroID, "", nil
		}
		return meta.ZeroID, "", fmt.Errorf("failed to find password credential: %w", err)
	}
	return po.ID, string(po.Material), nil
}

func credentialNotFoundError() error {
	return perrors.WithCode(code.ErrCredentialNotFound, "credential not found")
}

func (r *Repository) updateCredential(ctx context.Context, id meta.ID, updates map[string]interface{}, field string) error {
	result := r.WithContext(ctx).
		Model(&V2PO{}).
		Where("id = ?", id.Uint64()).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update credential %s: %w", field, result.Error)
	}
	if result.RowsAffected == 0 {
		return credentialNotFoundError()
	}
	return nil
}
