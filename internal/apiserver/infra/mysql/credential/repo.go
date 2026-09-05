package credential

import (
	"context"
	"fmt"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (r *Repository) UpdateStatus(ctx context.Context, id meta.ID, status domain.CredentialStatus) error {
	return r.updateCredential(ctx, id, map[string]interface{}{"status": status.String()}, "status")
}

func (r *Repository) ApplyAuthenticationTransition(
	ctx context.Context,
	transition domain.AuthenticationTransition,
) (domain.AuthenticationState, error) {
	switch transition.Kind {
	case domain.TransitionRecordFailure:
		return r.RecordAuthenticationFailure(ctx, transition.CredentialID, transition.Now, transition.LockoutPolicy)
	case domain.TransitionRecordSuccess:
		err := r.RecordAuthenticationSuccess(ctx, transition.CredentialID, transition.Now, transition.Rotation)
		return domain.AuthenticationState{}, err
	default:
		return domain.AuthenticationState{}, nil
	}
}

func (r *Repository) RecordAuthenticationFailure(
	ctx context.Context,
	id meta.ID,
	now time.Time,
	policy domain.LockoutPolicy,
) (domain.AuthenticationState, error) {
	if id.IsZero() {
		return domain.AuthenticationState{}, perrors.WithCode(code.ErrInvalidArgument, "credential id is required")
	}
	now = now.UTC()
	var state domain.AuthenticationState
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Select("failed_attempts", "locked_until").Where("id = ?", id.Uint64())
		if tx.Dialector.Name() == "mysql" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		var before V2PO
		if err := query.First(&before).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return credentialNotFoundError()
			}
			return fmt.Errorf("lock credential authentication state: %w", err)
		}
		// 在行锁保护下使用领域迁移计算状态；仓储只持久化结果，避免 SQL
		// 重复实现锁定规则及不同数据库的赋值顺序差异。
		credential := r.mapper.ToDO(&before)
		state = domain.ApplyAuthenticationTransition(credential, domain.NewFailureTransition(id, now, policy))
		updates := map[string]interface{}{
			"failed_attempts": credential.FailedAttempts,
			"last_failure_at": credential.LastFailureAt,
			"locked_until":    credential.LockedUntil,
		}
		result := tx.Model(&V2PO{}).Where("id = ?", id.Uint64()).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("record credential authentication failure: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return credentialNotFoundError()
		}

		return nil
	})
	return state, err
}

func (r *Repository) RecordAuthenticationSuccess(
	ctx context.Context,
	id meta.ID,
	now time.Time,
	rotation *domain.MaterialRotation,
) error {
	if id.IsZero() {
		return perrors.WithCode(code.ErrInvalidArgument, "credential id is required")
	}
	updates := map[string]interface{}{
		"failed_attempts": 0,
		"last_success_at": now.UTC(),
	}
	if rotation != nil && len(rotation.Material) > 0 {
		updates["material"] = rotation.Material
		if rotation.Algo != nil {
			updates["algo"] = *rotation.Algo
		}
	}
	return r.updateCredential(ctx, id, updates, "authentication_success")
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
