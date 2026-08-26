package loginidentity

import (
	"context"
	"fmt"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	linking "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/linking"
	authn "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	if err := r.createAndSync(ctx, po, identity); err == nil {
		return nil
	} else if po.GlobalIdentifier == nil || !perrors.IsCode(err, code.ErrLoginIdentityExists) {
		return fmt.Errorf("failed to create login identity: %w", err)
	}

	globalIdentifier := *po.GlobalIdentifier
	existing, err := r.getByGlobalIdentifierForConflict(ctx, domain.Provider(po.Provider), globalIdentifier)
	if err != nil {
		return fmt.Errorf("load canonical global login identity: %w", err)
	}
	if existing == nil {
		return perrors.WithCode(code.ErrLoginIdentityExists, "login identity already exists")
	}
	if existing.UserID != identity.UserID {
		return perrors.WithCode(code.ErrGlobalIdentifierExists, "global identifier already belongs to another user")
	}
	if !existing.IsActive() {
		return r.moveInactiveGlobalIdentifier(ctx, po, identity, globalIdentifier)
	}

	// 同一 User 的其他 realm LoginIdentity 不重复保存 global_identifier；
	// 唯一的 canonical 行已经承担跨 realm 查找锚点。
	po.GlobalIdentifier = nil
	if err := r.createAndSync(ctx, po, identity); err != nil {
		return fmt.Errorf("failed to create login identity without duplicate global identifier: %w", err)
	}
	return nil
}

// getByGlobalIdentifierForConflict uses a current/locking read on MySQL. A
// signup transaction may already own an older REPEATABLE READ snapshot; after
// a duplicate-key wait, a plain SELECT could therefore miss the transaction
// that just won the unique index race.
func (r *Repository) getByGlobalIdentifierForConflict(
	ctx context.Context,
	provider domain.Provider,
	globalIdentifier string,
) (*domain.LoginIdentity, error) {
	var po PO
	query := r.WithContext(ctx).
		Where("provider = ? AND global_identifier = ?", string(provider), strings.TrimSpace(globalIdentifier))
	if query.Dialector.Name() == "mysql" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return r.mapper.ToDO(&po), nil
}

func (r *Repository) createAndSync(ctx context.Context, po *PO, identity *domain.LoginIdentity) error {
	return r.CreateAndSync(ctx, po, func(updated *PO) {
		identity.ID = updated.ID
		identity.CreatedAt = updated.CreatedAt
		identity.UpdatedAt = updated.UpdatedAt
		if updated.GlobalIdentifier == nil {
			identity.GlobalIdentifier = ""
		} else {
			identity.GlobalIdentifier = *updated.GlobalIdentifier
		}
	})
}

func (r *Repository) moveInactiveGlobalIdentifier(
	ctx context.Context,
	po *PO,
	identity *domain.LoginIdentity,
	globalIdentifier string,
) error {
	uow := mysql.NewUnitOfWork(r.db)
	err := uow.WithinTransaction(ctx, func(txCtx context.Context) error {
		var anchor PO
		query := r.WithContext(txCtx).
			Where("provider = ? AND global_identifier = ?", po.Provider, globalIdentifier)
		if r.WithContext(txCtx).Dialector.Name() == "mysql" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&anchor).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return r.createAndSync(txCtx, po, identity)
			}
			return fmt.Errorf("lock canonical global login identity: %w", err)
		}
		if anchor.UserID != identity.UserID {
			return perrors.WithCode(code.ErrGlobalIdentifierExists, "global identifier already belongs to another user")
		}
		if domain.Status(anchor.Status) == domain.StatusActive {
			po.GlobalIdentifier = nil
			return r.createAndSync(txCtx, po, identity)
		}
		result := r.WithContext(txCtx).
			Model(&PO{}).
			Where("id = ? AND global_identifier = ?", anchor.ID, globalIdentifier).
			Update("global_identifier", nil)
		if result.Error != nil {
			return fmt.Errorf("release inactive canonical global login identity: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("release inactive canonical global login identity: anchor changed")
		}
		return r.createAndSync(txCtx, po, identity)
	})
	if err != nil {
		if perrors.IsCode(err, code.ErrGlobalIdentifierExists) {
			return err
		}
		return fmt.Errorf("move canonical global login identity: %w", err)
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
		return perrors.WithCode(code.ErrLoginIdentityNotFound, "login identity not found")
	}
	return nil
}

func (r *Repository) UnlinkOwnedUnlessLastActive(
	ctx context.Context,
	userID meta.ID,
	loginIdentityID meta.ID,
) (linking.UnlinkOutcome, error) {
	var outcome linking.UnlinkOutcome
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("user_id = ?", userID).
			Order("user_id ASC, id ASC")
		if tx.Dialector.Name() == "mysql" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		var identities []PO
		if err := query.Find(&identities).Error; err != nil {
			return fmt.Errorf("lock login identities for unlink: %w", err)
		}

		var target *PO
		activeCount := 0
		for i := range identities {
			if domain.Status(identities[i].Status) == domain.StatusActive {
				activeCount++
			}
			if identities[i].ID == loginIdentityID {
				target = &identities[i]
			}
		}
		if target == nil {
			outcome = linking.UnlinkOutcomeNotFound
			return nil
		}
		if domain.Status(target.Status) == domain.StatusActive && activeCount <= 1 {
			outcome = linking.UnlinkOutcomeLastActive
			return nil
		}
		if target.GlobalIdentifier != nil {
			var replacement *PO
			for i := range identities {
				candidate := &identities[i]
				if candidate.ID != target.ID &&
					candidate.Provider == target.Provider &&
					domain.Status(candidate.Status) == domain.StatusActive {
					replacement = candidate
					break
				}
			}
			if replacement != nil {
				globalIdentifier := *target.GlobalIdentifier
				if err := tx.Model(&PO{}).
					Where("id = ? AND global_identifier = ?", target.ID, globalIdentifier).
					Update("global_identifier", nil).Error; err != nil {
					return fmt.Errorf("release canonical global identifier during unlink: %w", err)
				}
				result := tx.Model(&PO{}).
					Where("id = ? AND global_identifier IS NULL", replacement.ID).
					Update("global_identifier", globalIdentifier)
				if result.Error != nil {
					return fmt.Errorf("transfer canonical global identifier during unlink: %w", result.Error)
				}
				if result.RowsAffected != 1 {
					return fmt.Errorf("transfer canonical global identifier during unlink: replacement changed")
				}
			}
		}
		result := tx.Model(&PO{}).
			Where("user_id = ? AND id = ?", userID, loginIdentityID).
			Update("status", string(domain.StatusDeleted))
		if result.Error != nil {
			return fmt.Errorf("unlink login identity: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			outcome = linking.UnlinkOutcomeNotFound
			return nil
		}
		outcome = linking.UnlinkOutcomeUnlinked
		return nil
	})
	return outcome, err
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

// FindLegacyWechatIdentityByProviderKey resolves only identities explicitly
// marked as originating from the legacy OAuth identifier column. The old
// column stored either OpenID or UnionID, while the canonical model separates
// them. Keeping this fallback marker-scoped prevents ordinary identities from
// acquiring broader lookup semantics.
func (r *Repository) FindLegacyWechatIdentityByProviderKey(
	ctx context.Context,
	provider domain.Provider,
	realm string,
	identifier string,
) (*authn.LoginIdentityLookup, error) {
	identity, err := r.GetByProviderKey(ctx, provider, realm, identifier)
	if err != nil || identity == nil {
		return nil, err
	}
	if identity.Meta[domain.MetaLegacyIdentifierSemantics] != domain.LegacyIdentifierOpenOrUnion {
		return nil, nil
	}
	return toLookup(identity), nil
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
