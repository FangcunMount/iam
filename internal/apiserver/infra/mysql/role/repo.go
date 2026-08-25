package role

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RoleRepository MySQL 实现
type RoleRepository struct {
	mysql.BaseRepository[*RolePO]
	mapper *Mapper
	db     *gorm.DB
}

var _ domain.Repository = (*RoleRepository)(nil)

// NewRoleRepository 构造函数
func NewRoleRepository(db *gorm.DB) domain.Repository {
	base := mysql.NewBaseRepository[*RolePO](db)
	base.SetErrorTranslator(mysql.NewDuplicateToTranslator(func(e error) error {
		return perrors.WithCode(code.ErrRoleAlreadyExists, "role already exists")
	}))

	return &RoleRepository{
		BaseRepository: base,
		mapper:         NewMapper(),
		db:             db,
	}
}

// Create 创建新角色
func (r *RoleRepository) Create(ctx context.Context, role *domain.Role) error {
	po := r.mapper.ToRolePO(role)
	return r.BaseRepository.CreateAndSync(ctx, po, func(updated *RolePO) {
		role.ID = updated.ID
	})
}

// Update 更新角色
func (r *RoleRepository) Update(ctx context.Context, role *domain.Role) error {
	po := r.mapper.ToRolePO(role)
	return r.BaseRepository.UpdateAndSync(ctx, po, func(updated *RolePO) {
		// Sync if needed
	})
}

// Delete 删除角色
func (r *RoleRepository) Delete(ctx context.Context, id meta.ID) error {
	return r.BaseRepository.DeleteByID(ctx, id.Uint64())
}

// FindByID 根据ID获取角色
func (r *RoleRepository) FindByID(ctx context.Context, id meta.ID) (*domain.Role, error) {
	return r.findByID(ctx, id, false)
}

func (r *RoleRepository) FindByIDForUpdate(ctx context.Context, id meta.ID) (*domain.Role, error) {
	return r.findByID(ctx, id, true)
}

func (r *RoleRepository) findByID(ctx context.Context, id meta.ID, lock bool) (*domain.Role, error) {
	var po RolePO
	query := r.WithContext(ctx)
	if lock && r.db != nil && r.db.Dialector != nil && r.db.Dialector.Name() != "sqlite" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&po, id.Uint64()).Error; err != nil {
		return nil, err
	}
	role, err := r.mapper.ToRoleBO(&po)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return role, nil
}

// FindByName 根据名称和租户获取角色
func (r *RoleRepository) FindByName(ctx context.Context, tenantID, name string) (*domain.Role, error) {
	var po RolePO
	err := r.WithContext(ctx).Where("tenant_id = ? AND name = ?", tenantID, name).First(&po).Error
	if err != nil {
		return nil, err
	}
	role, err := r.mapper.ToRoleBO(&po)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return role, nil
}

// List 列出角色
func (r *RoleRepository) List(ctx context.Context, tenantID string, offset, limit int) ([]*domain.Role, int64, error) {
	var pos []*RolePO
	var total int64

	query := r.WithContext(ctx).Model(&RolePO{}).Where("tenant_id = ?", tenantID)

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取分页数据
	if err := query.Offset(offset).Limit(limit).Find(&pos).Error; err != nil {
		return nil, 0, err
	}

	roles := make([]*domain.Role, 0, len(pos))
	for _, po := range pos {
		role, err := r.mapper.ToRoleBO(po)
		if err != nil {
			return nil, 0, err
		}
		if role != nil {
			roles = append(roles, role)
		}
	}

	return roles, total, nil
}
