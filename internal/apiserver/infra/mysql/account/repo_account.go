package account

import (
	"context"
	"fmt"
	"strings"

	"github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	authn "github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"gorm.io/gorm"
)

// AccountRepository 账号仓储实现（基于 BaseRepository）
type AccountRepository struct {
	mysql.BaseRepository[*AccountPO]
	mapper *Mapper
}

// NewAccountRepository 创建账号仓储实例
func NewAccountRepository(db *gorm.DB) *AccountRepository {
	base := mysql.NewBaseRepository[*AccountPO](db)
	base.SetErrorTranslator(mysql.NewDuplicateToTranslator(func(e error) error {
		return errors.WithCode(code.ErrAccountExists, "account already exists")
	}))

	return &AccountRepository{
		BaseRepository: base,
		mapper:         NewMapper(),
	}
}

// ==================== 创建 ====================

// Create 创建账号
func (r *AccountRepository) Create(ctx context.Context, acc *domain.Account) error {
	po := r.mapper.ToAccountPO(acc)
	if err := r.CreateAndSync(ctx, po, func(updated *AccountPO) {
		id := meta.FromUint64(updated.ID.Uint64()) // ID 来自数据库，必定有效
		acc.ID = id
	}); err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}
	return nil
}

// ==================== 更新 ====================

// UpdateUniqueID 更新唯一标识
func (r *AccountRepository) UpdateUniqueID(ctx context.Context, id meta.ID, uniqueID domain.UnionID) error {
	uniqueIDStr := string(uniqueID)
	result := r.WithContext(ctx).
		Model(&AccountPO{}).
		Where("id = ?", id.Uint64()).
		Update("unique_id", uniqueIDStr)

	if result.Error != nil {
		return fmt.Errorf("failed to update account unique_id: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateStatus 更新账号状态
func (r *AccountRepository) UpdateStatus(ctx context.Context, id meta.ID, status domain.AccountStatus) error {
	result := r.WithContext(ctx).
		Model(&AccountPO{}).
		Where("id = ?", id.Uint64()).
		Update("status", int8(status))

	if result.Error != nil {
		return fmt.Errorf("failed to update account status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateProfile 更新用户资料
func (r *AccountRepository) UpdateProfile(ctx context.Context, id meta.ID, profile map[string]string) error {
	profileJSON := mapToJSON(profile)
	result := r.WithContext(ctx).
		Model(&AccountPO{}).
		Where("id = ?", id.Uint64()).
		Update("profile", profileJSON)

	if result.Error != nil {
		return fmt.Errorf("failed to update account profile: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateMeta 更新元数据
func (r *AccountRepository) UpdateMeta(ctx context.Context, id meta.ID, metaData map[string]string) error {
	metaJSON := mapToJSON(metaData)
	result := r.WithContext(ctx).
		Model(&AccountPO{}).
		Where("id = ?", id.Uint64()).
		Update("meta", metaJSON)

	if result.Error != nil {
		return fmt.Errorf("failed to update account meta: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ==================== 查询 ====================

// GetByID 根据ID查询账号
func (r *AccountRepository) GetByID(ctx context.Context, id meta.ID) (*domain.Account, error) {
	var po AccountPO
	if err := r.WithContext(ctx).
		Where("id = ?", id.Uint64()).
		First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get account by id: %w", err)
	}
	return r.mapper.ToAccountDO(&po), nil
}

// GetByUniqueID 根据唯一标识查询账号
func (r *AccountRepository) GetByUniqueID(ctx context.Context, uniqueID domain.UnionID) (*domain.Account, error) {
	var po AccountPO
	if err := r.WithContext(ctx).
		Where("unique_id = ?", string(uniqueID)).
		First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get account by unique_id: %w", err)
	}
	return r.mapper.ToAccountDO(&po), nil
}

// GetByExternalIDAppId 根据外部ID和AppID查询账号
func (r *AccountRepository) GetByExternalIDAppId(ctx context.Context, externalID domain.ExternalID, appID domain.AppId) (*domain.Account, error) {
	var po AccountPO
	query := r.WithContext(ctx).Where("external_id = ?", string(externalID))

	// AppID 可能为空
	if appID != "" {
		appIDStr := string(appID)
		query = query.Where("app_id = ?", appIDStr)
	} else {
		// 兼容历史 schema/数据：
		// - 旧表结构将 app_id 建模为 NOT NULL DEFAULT ''
		// - 新代码路径有时会按“空 AppID”理解为 NULL
		// 这里同时兼容两种存储形态，避免查重/复用失真。
		query = query.Where("(app_id = '' OR app_id IS NULL)")
	}

	if err := query.First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get account by external_id and app_id: %w", err)
	}
	return r.mapper.ToAccountDO(&po), nil
}

// ==================== 认证端口实现 ====================

// FindAccountByUsername 根据用户名查找账户（用于密码认证）
// 实现 wechatapp.AccountRepository 接口。登录名须与创建运营账号时写入的 auth_accounts.external_id 一致。
func (r *AccountRepository) FindAccountByUsername(ctx context.Context, tenantID meta.ID, username string) (*authn.UsernameLoginLookup, error) {
	_ = tenantID
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, nil
	}

	var po AccountPO
	if err := r.WithContext(ctx).Where("external_id = ?", username).First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find account by username: %w", err)
	}
	return &authn.UsernameLoginLookup{
		AccountID:      po.ID,
		UserID:         po.UserID,
		AccountType:    po.Type,
		ScopedTenantID: po.ScopedTenantID,
	}, nil
}

// GetAccountStatus 获取账户状态（用于检查账户是否锁定/禁用）
// 实现 wechatapp.AccountRepository 接口
func (r *AccountRepository) GetAccountStatus(ctx context.Context, accountID meta.ID) (enabled, locked bool, err error) {
	var po AccountPO

	if err := r.WithContext(ctx).
		Select("status").
		Where("id = ?", accountID).
		First(&po).Error; err != nil {
		return false, false, fmt.Errorf("failed to get account status: %w", err)
	}

	// status: 1=正常, 0=禁用, -1=锁定等
	// 根据实际的 AccountStatus 定义调整
	enabled = po.Status == 1
	locked = po.Status < 0

	return enabled, locked, nil
}
