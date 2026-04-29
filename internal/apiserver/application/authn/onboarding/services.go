package onboarding

import (
	"context"

	domain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	userDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ============= 应用服务接口（Driving Ports）=============

// AccountOnboarder 负责账号开通流程。
type AccountOnboarder interface {
	// Onboard 完成：1) 创建或复用 User 2) 创建 Account 3) 绑定 Credential 4) 返回账号开通结果。
	Onboard(ctx context.Context, req OnboardingRequest) (*OnboardingResult, error)
}

// ============= DTOs =============

// OnboardingRequest 统一账号开通请求
type OnboardingRequest struct {
	// ========== 用户基本信息（必须）==========
	Name  string     // 用户姓名
	Phone meta.Phone // 手机号（E.164格式）
	Email meta.Email // 邮箱（可选）

	// ExistingUserID 非零时为已有用户绑定账号（不创建 User），用于 seed、管理端等场景。
	ExistingUserID meta.ID

	// OperaLoginID 运营账号写入 auth_accounts.external_id 的登录名，与密码登录 username 一致；空则由领域按邮箱/手机号推导。
	OperaLoginID string

	// ScopedTenantID TypeOpera 时必填：运营账号所属租户 ID；其它账户类型须为 0。
	ScopedTenantID meta.ID

	// ========== 账户类型（必须）==========
	AccountType domain.AccountType // 账户类型（决定注册流程，如微信小程序需要 code2session）

	// ========== 凭据类型（必须）==========
	CredentialType CredentialType // 凭据类型（决定绑定哪种凭据）

	// ========== 密码凭据参数 ==========
	Password *string // 密码（当 CredentialType = password 时必须）

	// ========== 微信小程序账户参数 ==========
	WechatAppID   *string // 微信AppID（当 AccountType = TypeWcMinip 时必须）
	WechatJsCode  *string // 微信JsCode（用于 code2session，AppSecret 由服务端根据 AppID 查询）
	WechatOpenID  *string // 微信OpenID（可选，如果有就不需要 code2session）
	WechatUnionID *string // 微信UnionID（可选）

	// ========== 企业微信账户参数 ==========
	WecomCorpID *string // 企业CorpID（当 AccountType = TypeWcCom 时必须）
	WecomUserID *string // 企业微信UserID（当 AccountType = TypeWcCom 时必须）

	// ========== 账户元数据（可选）==========
	Profile    map[string]string // 用户资料（昵称、头像等）
	Meta       map[string]string // 额外元数据
	ParamsJSON []byte            // 第三方平台用户信息JSON
}

// CredentialType 凭据类型
type CredentialType string

const (
	CredTypePassword CredentialType = "password" // 密码
	CredTypePhone    CredentialType = "phone"    // 手机号OTP
	CredTypeWechat   CredentialType = "wechat"   // 微信小程序
	CredTypeWecom    CredentialType = "wecom"    // 企业微信
)

// OnboardingResult 账号开通结果
type OnboardingResult struct {
	// 用户信息
	UserID     meta.ID               // 用户ID
	UserName   string                // 用户姓名
	Phone      meta.Phone            // 手机号
	Email      meta.Email            // 邮箱
	UserStatus userDomain.UserStatus // 用户状态

	// 账户信息
	AccountID   meta.ID            // 账户ID
	AccountType domain.AccountType // 账户类型
	ExternalID  domain.ExternalID  // 外部标识

	// 凭据信息
	CredentialID meta.ID // 凭据ID

	// 状态
	IsNewUser    bool // 是否新建用户（true=新建，false=已存在）
	IsNewAccount bool // 是否新建账户（true=新建，false=已存在）
}
