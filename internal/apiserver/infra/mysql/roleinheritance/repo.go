package roleinheritance

import (
	"context"
	"errors"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	mysql.BaseRepository[*InheritancePO]
	mapper Mapper
}

var _ domain.Repository = (*Repository)(nil)

func NewRepository(db *gorm.DB) domain.Repository {
	base := mysql.NewBaseRepository[*InheritancePO](db)
	base.SetErrorTranslator(mysql.NewDuplicateToTranslator(func(error) error {
		return perrors.WithCode(code.ErrRoleInheritanceAlreadyExists, "role inheritance already exists")
	}))
	return &Repository{BaseRepository: base}
}

func (r *Repository) Create(ctx context.Context, inheritance *domain.Inheritance) error {
	return r.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasTable("authz_roles") {
			var roleIDs []uint64
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Table("authz_roles").
				Where("tenant_id = ? AND deleted_at IS NULL", inheritance.TenantIDString()).
				Order("id ASC").
				Pluck("id", &roleIDs).Error; err != nil {
				return err
			}
			foundRole, foundInheritedRole := false, false
			for _, roleID := range roleIDs {
				foundRole = foundRole || roleID == inheritance.RoleID.Uint64()
				foundInheritedRole = foundInheritedRole || roleID == inheritance.InheritedRoleID.Uint64()
			}
			if !foundRole || !foundInheritedRole {
				return perrors.WithCode(code.ErrInvalidArgument, "role inheritance references an unknown or cross-tenant role")
			}
		}
		var rows []*InheritancePO
		query := tx.Where("tenant_id = ? AND revoked_at IS NULL", inheritance.TenantIDString()).Order("id ASC")
		if tx.Dialector != nil && tx.Dialector.Name() != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Find(&rows).Error; err != nil {
			return err
		}
		existing := make([]*domain.Inheritance, 0, len(rows))
		for _, row := range rows {
			edge, err := r.mapper.ToBO(row)
			if err != nil {
				return err
			}
			existing = append(existing, edge)
		}
		if domain.WouldCreateCycle(existing, inheritance.RoleID, inheritance.InheritedRoleID) {
			return perrors.WithCode(code.ErrInvalidArgument, "role inheritance would create a cycle")
		}
		po := r.mapper.ToPO(inheritance)
		if err := tx.Create(po).Error; err != nil {
			return mysql.NewDuplicateToTranslator(func(error) error {
				return perrors.WithCode(code.ErrRoleInheritanceAlreadyExists, "role inheritance already exists")
			})(err)
		}
		inheritance.ID = po.ID
		inheritance.GrantedAt = po.GrantedAt
		inheritance.Version = po.Version
		return nil
	})
}

func (r *Repository) AtomicRevoke(ctx context.Context, id meta.ID, tenantID string) (domain.RevokeOutcome, error) {
	now := time.Now()
	query := r.WithContext(ctx).Model(&InheritancePO{}).
		Where("id = ? AND revoked_at IS NULL", id.Uint64())
	if strings.TrimSpace(tenantID) != "" {
		query = query.Where("tenant_id = ?", strings.TrimSpace(tenantID))
	}
	result := query.Updates(map[string]any{
		"revoked_at": now,
		"updated_at": now,
		"updated_by": mysql.UserIDOrZero(ctx).Uint64(),
		"version":    gorm.Expr("version + 1"),
	})
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 1 {
		return domain.RevokeOutcomeRevoked, nil
	}
	var po InheritancePO
	findQuery := r.WithContext(ctx)
	if findQuery.Dialector != nil && findQuery.Dialector.Name() != "sqlite" {
		findQuery = findQuery.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	findQuery = findQuery.Where("id = ?", id.Uint64())
	if strings.TrimSpace(tenantID) != "" {
		findQuery = findQuery.Where("tenant_id = ?", strings.TrimSpace(tenantID))
	}
	if err := findQuery.First(&po).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.RevokeOutcomeNotFound, nil
		}
		return "", err
	}
	if po.RevokedAt != nil {
		return domain.RevokeOutcomeAlreadyRevoked, nil
	}
	return domain.RevokeOutcomeNotFound, nil
}

func (r *Repository) FindByID(ctx context.Context, id meta.ID) (*domain.Inheritance, error) {
	po, err := r.BaseRepository.FindByID(ctx, id.Uint64())
	if err != nil {
		return nil, err
	}
	return r.mapper.ToBO(po)
}

func (r *Repository) ListActiveByTenant(ctx context.Context, tenantID string) ([]*domain.Inheritance, error) {
	var rows []*InheritancePO
	if err := r.WithContext(ctx).
		Where("tenant_id = ? AND revoked_at IS NULL", tenantID).
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]*domain.Inheritance, 0, len(rows))
	for _, row := range rows {
		inheritance, err := r.mapper.ToBO(row)
		if err != nil {
			return nil, err
		}
		result = append(result, inheritance)
	}
	return result, nil
}
