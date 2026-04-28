package credential

import (
	"context"
	"fmt"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"gorm.io/gorm"
)

// Repository 凭据仓储实现（基于 BaseRepository）。
type Repository struct {
	mysql.BaseRepository[*PO]
	db     *gorm.DB
	mapper *Mapper
}

// NewRepository 创建凭据仓储实例。
func NewRepository(db *gorm.DB) *Repository {
	base := mysql.NewBaseRepository[*PO](db)
	// 当出现唯一约束冲突时，把 DB 错误翻译为业务错误码 ErrCredentialExists
	base.SetErrorTranslator(mysql.NewDuplicateToTranslator(func(e error) error {
		return perrors.WithCode(code.ErrCredentialExists, "credential already exists")
	}))

	return &Repository{
		BaseRepository: base,
		db:             db,
		mapper:         NewMapper(),
	}
}

// Create 创建凭据。
func (r *Repository) Create(ctx context.Context, cred *domain.Credential) error {
	po := r.mapper.ToPO(cred)
	return r.CreateAndSync(ctx, po, func(updated *PO) {
		cred.ID = updated.ID
	})
}

// UpdateMaterial 更新凭据材料（用于密码重置、轮换等）。
func (r *Repository) UpdateMaterial(ctx context.Context, id meta.ID, material []byte, algo string) error {
	result := r.db.WithContext(ctx).
		Model(&PO{}).
		Where("id = ?", id.Uint64()).
		Updates(map[string]interface{}{
			"material": material,
			"algo":     algo,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update credential material: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return credentialNotFoundError()
	}
	return nil
}

// UpdateStatus 更新凭据状态。
func (r *Repository) UpdateStatus(ctx context.Context, id meta.ID, status domain.CredentialStatus) error {
	result := r.db.WithContext(ctx).
		Model(&PO{}).
		Where("id = ?", id.Uint64()).
		Update("status", int8(status))

	if result.Error != nil {
		return fmt.Errorf("failed to update credential status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return credentialNotFoundError()
	}
	return nil
}

// UpdateFailedAttempts 更新失败尝试次数（用于账号锁定策略）。
func (r *Repository) UpdateFailedAttempts(ctx context.Context, id meta.ID, attempts int) error {
	result := r.db.WithContext(ctx).
		Model(&PO{}).
		Where("id = ?", id.Uint64()).
		Update("failed_attempts", attempts)

	if result.Error != nil {
		return fmt.Errorf("failed to update credential failed_attempts: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return credentialNotFoundError()
	}
	return nil
}

// UpdateLockedUntil 更新锁定截止时间。
func (r *Repository) UpdateLockedUntil(ctx context.Context, id meta.ID, lockedUntil *time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&PO{}).
		Where("id = ?", id.Uint64()).
		Update("locked_until", lockedUntil)

	if result.Error != nil {
		return fmt.Errorf("failed to update credential locked_until: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return credentialNotFoundError()
	}
	return nil
}

// UpdateLastSuccessAt 更新最近成功时间。
func (r *Repository) UpdateLastSuccessAt(ctx context.Context, id meta.ID, lastSuccessAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&PO{}).
		Where("id = ?", id.Uint64()).
		Update("last_success_at", lastSuccessAt)

	if result.Error != nil {
		return fmt.Errorf("failed to update credential last_success_at: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return credentialNotFoundError()
	}
	return nil
}

// UpdateLastFailureAt 更新最近失败时间。
func (r *Repository) UpdateLastFailureAt(ctx context.Context, id meta.ID, lastFailureAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&PO{}).
		Where("id = ?", id.Uint64()).
		Update("last_failure_at", lastFailureAt)

	if result.Error != nil {
		return fmt.Errorf("failed to update credential last_failure_at: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return credentialNotFoundError()
	}
	return nil
}

// UpdateExpiresAt 更新过期时间（当前 PO 未定义此字段，返回未实现错误）。
func (r *Repository) UpdateExpiresAt(ctx context.Context, id meta.ID, expiresAt *time.Time) error {
	return fmt.Errorf("UpdateExpiresAt not implemented: expires_at field not defined in credential PO")
}

// GetByID 根据ID查询凭据。
func (r *Repository) GetByID(ctx context.Context, id meta.ID) (*domain.Credential, error) {
	var po PO
	if err := r.db.WithContext(ctx).
		Where("id = ?", id.Uint64()).
		First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get credential by id: %w", err)
	}
	return r.mapper.ToDO(&po), nil
}

// GetByAccountIDAndType 根据账号ID和类型查找凭据（接口方法）。
func (r *Repository) GetByAccountIDAndType(ctx context.Context, accountID meta.ID, credType domain.CredentialType) (*domain.Credential, error) {
	var po PO
	if err := r.db.WithContext(ctx).
		Where("account_id = ? AND type = ?", accountID.Uint64(), string(credType)).
		First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get credential by account/type: %w", err)
	}
	return r.mapper.ToDO(&po), nil
}

// GetPhoneOTP 获取手机号 OTP 凭据。
func (r *Repository) GetPhoneOTP(ctx context.Context, accountID int64, phone string) (*domain.Credential, error) {
	var po PO
	if err := r.db.WithContext(ctx).
		Where("account_id = ? AND idp = ? AND idp_identifier = ?", accountID, "phone", phone).
		First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get phone otp credential: %w", err)
	}
	return r.mapper.ToDO(&po), nil
}

// ListByAccountID 列出账号下的所有凭据（接口方法）。
func (r *Repository) ListByAccountID(ctx context.Context, accountID meta.ID) ([]*domain.Credential, error) {
	var pos []PO
	if err := r.db.WithContext(ctx).
		Where("account_id = ?", accountID.Uint64()).
		Find(&pos).Error; err != nil {
		return nil, fmt.Errorf("failed to list credentials by account: %w", err)
	}

	result := make([]*domain.Credential, 0, len(pos))
	for i := range pos {
		result = append(result, r.mapper.ToDO(&pos[i]))
	}
	return result, nil
}

// Delete 删除指定ID的凭据（接口方法）。
func (r *Repository) Delete(ctx context.Context, id meta.ID) error {
	result := r.db.WithContext(ctx).
		Where("id = ?", id.Uint64()).
		Delete(&PO{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete credential: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return credentialNotFoundError()
	}
	return nil
}

// GetByIDPIdentifier 根据外部身份标识和凭据类型查找凭据（接口方法）。
func (r *Repository) GetByIDPIdentifier(ctx context.Context, idpIdentifier string, credType domain.CredentialType) (*domain.Credential, error) {
	var po PO
	if err := r.db.WithContext(ctx).
		Where("idp_identifier = ? AND type = ?", idpIdentifier, string(credType)).
		First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get credential by idp identifier: %w", err)
	}
	return r.mapper.ToDO(&po), nil
}

// FindPasswordCredential 根据账户ID查找密码凭据
// 返回：凭据ID、密码哈希值（PHC格式）
func (r *Repository) FindPasswordCredential(ctx context.Context, accountID meta.ID) (credentialID meta.ID, passwordHash string, err error) {
	var po PO
	if err := r.db.WithContext(ctx).
		Select("id", "material").
		Where("account_id = ? AND type = ?", accountID, "password").
		First(&po).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return meta.ZeroID, "", nil
		}
		return meta.ZeroID, "", fmt.Errorf("failed to find password credential: %w", err)
	}
	return po.ID, string(po.Material), nil
}

// FindPhoneOTPCredential 根据手机号查找OTP凭据绑定
// 返回：账户ID、用户ID、凭据ID
func (r *Repository) FindPhoneOTPCredential(ctx context.Context, phoneE164 string) (accountID, userID, credentialID meta.ID, err error) {
	type Result struct {
		CredentialID uint64 `gorm:"column:credential_id"`
		AccountID    uint64 `gorm:"column:account_id"`
		UserID       uint64 `gorm:"column:user_id"`
	}

	var results []Result
	err = r.db.WithContext(ctx).
		Table("auth_credentials c").
		Select("c.id as credential_id", "c.account_id", "a.user_id").
		Joins("INNER JOIN auth_accounts a ON c.account_id = a.id").
		Where("c.type = ? AND c.idp = ? AND c.idp_identifier = ?", "phone_otp", "phone", phoneE164).
		Order("c.id").
		Limit(1).
		Find(&results).Error

	if err != nil {
		zeroID := meta.FromUint64(0)
		return zeroID, zeroID, zeroID, fmt.Errorf("failed to find phone OTP credential: %w", err)
	}

	if len(results) == 0 {
		zeroID := meta.FromUint64(0)
		return zeroID, zeroID, zeroID, credentialNotFoundError()
	}

	result := results[0]

	accID := meta.FromUint64(result.AccountID)
	usrID := meta.FromUint64(result.UserID)
	credID := meta.FromUint64(result.CredentialID)
	return accID, usrID, credID, nil
}

func credentialNotFoundError() error {
	return perrors.WithCode(code.ErrCredentialNotFound, "credential not found")
}

// FindOAuthCredential 根据身份提供商标识查找OAuth凭据绑定
// idpType: "wx_minip" | "wecom" | ...
// idpIdentifier: OpenID/UnionID/UserID
// 返回：账户ID、用户ID、凭据ID
func (r *Repository) FindOAuthCredential(ctx context.Context, idpType, appID, idpIdentifier string) (accountID, userID, credentialID meta.ID, err error) {
	type Result struct {
		CredentialID uint64 `gorm:"column:credential_id"`
		AccountID    uint64 `gorm:"column:account_id"`
		UserID       uint64 `gorm:"column:user_id"`
	}

	var results []Result
	query := r.db.WithContext(ctx).
		Table("auth_credentials c").
		Select("c.id as credential_id", "c.account_id", "a.user_id").
		Joins("INNER JOIN auth_accounts a ON c.account_id = a.id").
		Where("c.type = ? AND c.idp_identifier = ?", idpType, idpIdentifier)

	// 如果提供了 appID,则增加 appID 过滤条件
	if appID != "" {
		query = query.Where("c.app_id = ?", appID)
	}

	err = query.Order("c.id").Limit(1).Find(&results).Error
	if err != nil {
		zeroID := meta.FromUint64(0)
		return zeroID, zeroID, zeroID, fmt.Errorf("failed to find OAuth credential: %w", err)
	}

	if len(results) == 0 {
		zeroID := meta.FromUint64(0)
		return zeroID, zeroID, zeroID, nil
	}

	result := results[0]

	accID := meta.FromUint64(result.AccountID)
	usrID := meta.FromUint64(result.UserID)
	credID := meta.FromUint64(result.CredentialID)
	return accID, usrID, credID, nil
}
