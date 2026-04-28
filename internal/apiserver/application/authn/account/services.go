package account

import (
	"context"

	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ============= 应用服务接口（Driving Ports）=============

// AccountApplicationService 账户应用服务 - 已存在账户的管理
type AccountApplicationService interface {
	// GetAccountByID 根据ID获取账户
	GetAccountByID(ctx context.Context, accountID meta.ID) (*AccountResult, error)

	// FindByExternalRef 根据外部引用查找账户
	// 用于幂等性检查和已有账户查询
	FindByExternalRef(ctx context.Context, accountType domain.AccountType, appID domain.AppId, externalID domain.ExternalID) (*AccountResult, error)

	// FindByUniqueID 根据全局唯一标识查找账户
	FindByUniqueID(ctx context.Context, uniqueID domain.UnionID) (*AccountResult, error)

	// SetUniqueID 设置全局唯一标识（如 UnionID）
	SetUniqueID(ctx context.Context, accountID meta.ID, uniqueID domain.UnionID) error

	// UpdateProfile 更新账户资料
	UpdateProfile(ctx context.Context, accountID meta.ID, profile map[string]string) error

	// UpdateMeta 更新账户元数据
	UpdateMeta(ctx context.Context, accountID meta.ID, meta map[string]string) error

	// EnableAccount 启用账户
	EnableAccount(ctx context.Context, accountID meta.ID) error

	// DisableAccount 禁用账户
	DisableAccount(ctx context.Context, accountID meta.ID) error

	// ArchiveAccount 归档账户
	ArchiveAccount(ctx context.Context, accountID meta.ID) error

	// DeleteAccount 删除账户（软删除）
	DeleteAccount(ctx context.Context, accountID meta.ID) error
}

// ============= DTOs =============

// AccountResult 账户结果DTO
type AccountResult struct {
	AccountID  meta.ID              // 账户ID
	UserID     meta.ID              // 用户ID
	Type       domain.AccountType   // 账户类型
	AppID      domain.AppId         // 应用ID
	ExternalID domain.ExternalID    // 外部标识
	UniqueID   domain.UnionID       // 全局唯一标识
	Profile    map[string]string    // 用户资料
	Meta       map[string]string    // 元数据
	Status     domain.AccountStatus // 账户状态
}
