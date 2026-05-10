package credential

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
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

func (r *Repository) UpdateAuthState(ctx context.Context, cred *domain.Credential) error {
	if cred == nil || cred.ID.IsZero() {
		return perrors.WithCode(code.ErrInvalidArgument, "credential id is required")
	}
	return r.updateCredential(ctx, cred.ID, map[string]interface{}{
		"failed_attempts": cred.FailedAttempts,
		"locked_until":    cred.LockedUntil,
		"last_success_at": cred.LastSuccessAt,
		"last_failure_at": cred.LastFailureAt,
	}, "auth_state")
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

func (r *Repository) FindPasswordCredentialByLoginIdentity(ctx context.Context, loginIdentityID meta.ID) (*authentication.PasswordCredentialLookup, error) {
	var po V2PO
	if err := r.WithContext(ctx).
		Select("id", "material", "status", "locked_until").
		Where("login_identity_id = ? AND type = ?", loginIdentityID.Uint64(), string(domain.CredPassword)).
		First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find password credential: %w", err)
	}
	return &authentication.PasswordCredentialLookup{
		CredentialID: po.ID,
		PasswordHash: string(po.Material),
		Status:       statusFromString(po.Status),
		LockedUntil:  po.LockedUntil,
	}, nil
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
