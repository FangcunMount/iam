package account

import (
	"fmt"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// Account 第三方登录账号实体
type Account struct {
	ID             meta.ID
	UserID         meta.ID           // 关联到用户中心
	Type           AccountType       // 平台类型：wc-minip, wc-offi, wc-com, opera
	AppID          AppId             // 应用 ID：公众号 appid | 小程序 appid | 企业微信 corpid | 运营后台可为空
	ExternalID     ExternalID        // 外部平台用户标识：公众号/小程序 openid | 企业微信 userid | 运营后台 username
	UniqueID       UnionID           // 全局唯一标识：公众号/小程序/企业微信 unionid | 运营后台为空
	ScopedTenantID meta.ID           // ScopedTenantID 仅 TypeOpera 时有效：运营身份所属租户（B 端作用域）；其它类型必须为 0。
	Profile        map[string]string // 用户资料：昵称、头像等
	Meta           map[string]string // 额外元数据
	Status         AccountStatus     // 账号状态：激活、禁用、存档、删除
}

// ==================== 工厂方法 ====================

// NewAccount 创建第三方登录账号实体
func NewAccount(uID meta.ID, t AccountType, eID ExternalID, opts ...AccountOption) *Account {
	// 创建账号实体
	acc := &Account{
		// 第一个参数表示关联的 UserID（调用方通常传入用户 ID），
		// 实际的 Account.ID 应由持久层生成或通过 WithID 明确设置。
		UserID:     uID,
		Type:       t,
		ExternalID: eID,
		Status:     StatusActive, // 默认激活
		Profile:    make(map[string]string),
		Meta:       make(map[string]string),
	}
	// 应用选项
	for _, opt := range opts {
		opt(acc)
	}
	return acc
}

// AccountOption 第三方登录账号选项
type AccountOption func(*Account)

func WithID(id meta.ID) AccountOption               { return func(a *Account) { a.ID = id } }
func WithAppID(appid AppId) AccountOption           { return func(a *Account) { a.AppID = appid } }
func WithExternalID(eid ExternalID) AccountOption   { return func(a *Account) { a.ExternalID = eid } }
func WithUnionID(uid UnionID) AccountOption         { return func(a *Account) { a.UniqueID = uid } }
func WithStatus(status AccountStatus) AccountOption { return func(a *Account) { a.Status = status } }
func WithProfile(profile map[string]string) AccountOption {
	return func(a *Account) { a.Profile = profile }
}
func WithMeta(meta map[string]string) AccountOption {
	return func(a *Account) { a.Meta = meta }
}

// WithScopedTenantID 设置运营账户租户作用域（仅应与 TypeOpera 同时使用）。
func WithScopedTenantID(tenantID meta.ID) AccountOption {
	return func(a *Account) { a.ScopedTenantID = tenantID }
}

// ValidateScopedTenantInvariant 校验 ScopedTenantID 与账户类型一致。
func (a *Account) ValidateScopedTenantInvariant() error {
	if a == nil {
		return fmt.Errorf("account is nil")
	}
	if a.Type == TypeOpera {
		if a.ScopedTenantID.IsZero() {
			return fmt.Errorf("opera account requires non-zero ScopedTenantID")
		}
		return nil
	}
	if !a.ScopedTenantID.IsZero() {
		return fmt.Errorf("ScopedTenantID is only valid for opera accounts")
	}
	return nil
}

// ==================== 状态查询方法 ====================

// IsActive 是否为激活状态
func (a *Account) IsActive() bool { return a.Status == StatusActive }

// IsDisabled 是否为禁用状态
func (a *Account) IsDisabled() bool { return a.Status == StatusDisabled }

// IsArchived 是否为归档状态
func (a *Account) IsArchived() bool { return a.Status == StatusArchived }

// IsDeleted 是否为删除状态
func (a *Account) IsDeleted() bool { return a.Status == StatusDeleted }

// HasUniqueID 是否已设置全局唯一标识
func (a *Account) HasUniqueID() bool { return a.UniqueID != "" }

// IsSameUser 是否属于同一用户
func (a *Account) IsSameUser(userID meta.ID) bool { return a.UserID == userID }

// IsType 是否为指定类型
func (a *Account) IsType(t AccountType) bool { return a.Type == t }

// ==================== 状态转换方法 ====================

// Activate 激活账号
func (a *Account) Activate() {
	a.Status = StatusActive
}

// Disable 禁用账号
func (a *Account) Disable() {
	a.Status = StatusDisabled
}

// Archive 归档账号
func (a *Account) Archive() {
	a.Status = StatusArchived
}

// Delete 删除账号（软删除）
func (a *Account) Delete() {
	a.Status = StatusDeleted
}

// ==================== 数据更新方法 ====================

// SetUniqueID 设置全局唯一标识
// 只允许设置一次，已设置则返回 false
func (a *Account) SetUniqueID(uid UnionID) bool {
	if a.HasUniqueID() {
		return false // 已经设置过，不允许修改
	}
	a.UniqueID = uid
	return true
}

// UpdateProfile 更新用户资料
// 使用 merge 策略，只更新提供的字段
func (a *Account) UpdateProfile(profile map[string]string) {
	if a.Profile == nil {
		a.Profile = make(map[string]string)
	}
	for k, v := range profile {
		a.Profile[k] = v
	}
}

// SetProfileField 设置单个资料字段
func (a *Account) SetProfileField(key, value string) {
	if a.Profile == nil {
		a.Profile = make(map[string]string)
	}
	a.Profile[key] = value
}

// GetProfileField 获取单个资料字段
func (a *Account) GetProfileField(key string) (string, bool) {
	if a.Profile == nil {
		return "", false
	}
	v, ok := a.Profile[key]
	return v, ok
}

// UpdateMeta 更新元数据
// 使用 merge 策略，只更新提供的字段
func (a *Account) UpdateMeta(meta map[string]string) {
	if a.Meta == nil {
		a.Meta = make(map[string]string)
	}
	for k, v := range meta {
		a.Meta[k] = v
	}
}

// SetMetaField 设置单个元数据字段
func (a *Account) SetMetaField(key, value string) {
	if a.Meta == nil {
		a.Meta = make(map[string]string)
	}
	a.Meta[key] = value
}

// GetMetaField 获取单个元数据字段
func (a *Account) GetMetaField(key string) (string, bool) {
	if a.Meta == nil {
		return "", false
	}
	v, ok := a.Meta[key]
	return v, ok
}

// ==================== 业务规则验证方法 ====================

// CanTransitionTo 是否可以转换到目标状态
func (a *Account) CanTransitionTo(targetStatus AccountStatus) bool {
	// 状态转换规则：
	// - Disabled -> Active, Archived, Deleted
	// - Active -> Disabled, Archived, Deleted
	// - Archived -> Active, Deleted
	// - Deleted -> 不可转换（终态）

	switch a.Status {
	case StatusDisabled:
		return targetStatus == StatusActive ||
			targetStatus == StatusArchived ||
			targetStatus == StatusDeleted ||
			targetStatus == StatusDisabled // 幂等

	case StatusActive:
		return targetStatus == StatusDisabled ||
			targetStatus == StatusArchived ||
			targetStatus == StatusDeleted ||
			targetStatus == StatusActive // 幂等

	case StatusArchived:
		return targetStatus == StatusActive ||
			targetStatus == StatusDeleted ||
			targetStatus == StatusArchived // 幂等

	case StatusDeleted:
		// 删除是终态，只允许幂等操作
		return targetStatus == StatusDeleted

	default:
		return false
	}
}

// CanSetUniqueID 是否可以设置全局唯一标识
func (a *Account) CanSetUniqueID() bool {
	return !a.HasUniqueID()
}

// CanUpdateProfile 是否可以更新资料（非删除状态即可）
func (a *Account) CanUpdateProfile() bool {
	return !a.IsDeleted()
}

// CanUpdateMeta 是否可以更新元数据（非删除状态即可）
func (a *Account) CanUpdateMeta() bool {
	return !a.IsDeleted()
}
