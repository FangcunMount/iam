package account

import (
	"context"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ==================== Driving Ports (驱动端口) ====================
// 这些接口由领域层实现，供应用层调用
// 遵循接口隔离原则，按职责细分

// Editor 账号编辑器（Driving Port）
// 职责：编辑账号信息
type Editor interface {
	// SetUniqueID 设置全局唯一标识
	SetUniqueID(ctx context.Context, accountID meta.ID, uniqueID UnionID) (*Account, error)
	// UpdateProfile 更新账号资料
	UpdateProfile(ctx context.Context, accountID meta.ID, profile map[string]string) (*Account, error)
	// UpdateMeta 更新账号元数据
	UpdateMeta(ctx context.Context, accountID meta.ID, meta map[string]string) (*Account, error)
}

// StatusManager 账号状态管理器（Driving Port）
// 职责：管理账号状态转换
type StatusManager interface {
	// Activate 激活账号
	Activate(ctx context.Context, accountID meta.ID) (*Account, error)
	// Disable 禁用账号
	Disable(ctx context.Context, accountID meta.ID) (*Account, error)
	// Archive 归档账号
	Archive(ctx context.Context, accountID meta.ID) (*Account, error)
	// Delete 删除账号
	Delete(ctx context.Context, accountID meta.ID) (*Account, error)
}

// ==================== 账户创建策略接口（领域服务） ====================

// CreatorStrategy 账户创建策略接口
// 每种账户类型（AccountType）都有自己的创建策略
// 策略职责：
// 1. 准备账户创建所需的数据
// 2. 创建账户实体
// 注意：凭据颁发不属于账户创建策略，由 credential 领域负责
type CreatorStrategy interface {
	// Kind 返回策略支持的账户类型
	Kind() AccountType

	// PrepareData 准备账户信息
	// 在创建账户前调用，用于：
	// - 验证必要参数
	// - 将应用层准备好的身份数据转换为账户创建参数
	// 返回：准备好的创建参数
	PrepareData(ctx context.Context, input CreationInput) (*CreationParams, error)

	// Create 创建账户
	// - 确定 AccountType、AppID、ExternalID
	// - 创建账户实体
	// 返回：创建的账户实体
	Create(ctx context.Context, params *CreationParams) (*Account, error)
}

// ==================== 创建输入（DTOs） ====================

// CreationInput 账户创建输入（统一的创建参数）
type CreationInput struct {
	// ========== 用户信息 ==========
	UserID meta.ID    // 用户ID（必须）
	Phone  meta.Phone // 手机号（可选；TypeOpera 时若未提供 OperaLoginID/Email 则仍需要用于 external_id）
	Email  meta.Email // 邮箱（可选）

	// OperaLoginID 运营账号登录名，写入 auth_accounts.external_id，与密码登录 credentials.username 一致。
	// 非空时优先作为 external_id；空则依次尝试 Email、Phone。
	OperaLoginID string

	// ========== 账户类型（必须）==========
	AccountType AccountType // 要创建的账户类型（决定使用哪个策略）

	// ScopedTenantID TypeOpera 时必填：运营账号所属租户；其它类型须为 0。
	ScopedTenantID meta.ID

	// ========== 微信小程序专用 ==========
	WechatAppID   *string // 微信AppID（TypeWcMinip 必须）
	WechatOpenID  *string // 微信OpenID（TypeWcMinip 必须，由应用层解析 code2session 后注入）
	WechatUnionID *string // 微信UnionID（可选）

	// ========== 企业微信专用 ==========
	WecomCorpID *string // 企业CorpID（TypeWcCom 必须）
	WecomUserID *string // 企业微信UserID（TypeWcCom 必须）

	// ========== 账户元数据（可选）==========
	Profile    map[string]string // 用户资料（昵称、头像等）
	Meta       map[string]string // 额外元数据
	ParamsJSON []byte            // 第三方平台用户信息JSON
}

// CreationParams 准备好的账户创建参数
// PrepareData 返回此结构，包含创建账户所需的所有信息
type CreationParams struct {
	// ========== 基础信息 ==========
	UserID      meta.ID     // 用户ID
	AccountType AccountType // 账户类型
	AppID       AppId       // 应用ID
	ExternalID  ExternalID  // 外部标识
	// ScopedTenantID 见 CreationInput（TypeOpera 必填）。
	ScopedTenantID meta.ID

	// ========== 第三方服务返回的额外信息 ==========
	OpenID   string                 // 微信 OpenID
	UnionID  string                 // 微信 UnionID
	Session  string                 // 微信 Session Key
	Metadata map[string]interface{} // 其他元数据

	// ========== 账户元数据 ==========
	Profile    map[string]string // 用户资料
	Meta       map[string]string // 额外元数据
	ParamsJSON []byte            // 第三方平台用户信息JSON
}

// ==================== 账户创建器（AccountCreator）====================

// AccountCreator 账户创建器（策略协调器）
// 职责：协调账户创建流程，使用策略模式处理不同类型账户的创建
type AccountCreator interface {
	// CreateAccount 创建账户
	// 流程：
	// 1. 根据 AccountType 选择策略
	// 2. 使用策略准备数据（PrepareData）
	// 3. 使用策略创建账户（Create）
	// 4. 持久化账户
	// 返回：创建的账户和创建参数（包含第三方服务返回的信息）
	CreateAccount(ctx context.Context, input CreationInput) (*Account, *CreationParams, error)
}
