package onboarding

import (
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ============= DTOs =============

// OnboardingRequest 统一登录身份开通请求
type OnboardingRequest struct {
	// Scenario 开通场景。
	Scenario OnboardingScenario

	// ========== 用户基本信息（必须）==========
	Name  string     // 用户姓名
	Phone meta.Phone // 手机号（E.164格式）
	Email meta.Email // 邮箱（可选）

	// ExistingUserID 非零时为已有用户绑定账号（不创建 User），用于 seed、管理端等场景。
	ExistingUserID meta.ID

	// OperaLoginID 运营登录名，与密码登录 username 一致；空则按邮箱/手机号推导。
	OperaLoginID string

	// ScopedTenantID OnboardOperaPassword 时必填：运营登录身份所属租户 ID；其它场景须为 0。
	ScopedTenantID meta.ID

	// ========== 密码凭据参数 ==========
	Password *string // 密码（需要 password credential 的场景必须）

	// ========== 微信小程序登录身份参数 ==========
	WechatAppID   *string // 微信AppID
	WechatJsCode  *string // 微信JsCode（用于 code2session，AppSecret 由服务端根据 AppID 查询）
	WechatOpenID  *string // 微信OpenID（可选，如果有就不需要 code2session）
	WechatUnionID *string // 微信UnionID（可选）

	// ========== 企业微信登录身份参数 ==========
	WecomCorpID *string // 企业CorpID
	WecomUserID *string // 企业微信UserID

	// ========== 登录身份元数据（可选）==========
	Profile    map[string]string // 用户资料（昵称、头像等）
	Meta       map[string]string // 额外元数据
	ParamsJSON []byte            // 第三方平台用户信息JSON
}

// OnboardingResult 登录身份开通结果
type OnboardingResult struct {
	// 用户信息
	UserID     meta.ID           // 用户ID
	UserName   string            // 用户姓名
	Phone      meta.Phone        // 手机号
	Email      meta.Email        // 邮箱
	UserStatus userDomain.Status // 用户状态

	// 登录身份信息
	LoginIdentityID meta.ID // 登录身份ID

	// 凭据信息
	CredentialID meta.ID // 凭据ID

	// 状态
	IsNewUser          bool // 是否新建用户（true=新建，false=已存在）
	IsNewLoginIdentity bool // 是否新建登录身份（true=新建，false=已存在）
}
