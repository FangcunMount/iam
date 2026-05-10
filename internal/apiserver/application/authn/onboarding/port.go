package onboarding

import (
	"context"

	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	loginidentityDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ============= 用例入口（Driving Port）=============

// LoginIdentityOnboarder 负责登录身份开通用例。
//
// 对外调用方只依赖这个入口；请求准备、用户解析、登录身份确保、凭据确保都属于用例内部流程。
type LoginIdentityOnboarder interface {
	// Onboard 完成：1) 准备登录身份数据 2) 解析或创建 User 3) 确保 LoginIdentity 存在 4) 按需确保 Credential 存在。
	Onboard(ctx context.Context, req OnboardingRequest) (*OnboardingResult, error)
}

// OnboardingRequest 统一登录身份开通请求。
type OnboardingRequest struct {
	User          OnboardingUserInput
	LoginIdentity OnboardingLoginIdentityInput
	Credential    *OnboardingCredentialInput
}

// OnboardingUserInput 用户输入
//
// Name、Phone、Email 是用户信息，必填。
type OnboardingUserInput struct {
	Name  string
	Phone meta.Phone
	Email meta.Email
}

// UsernameLoginIdentityInput 用户名+密码开通的登录身份输入。
type UsernameLoginIdentityInput struct {
	Username string

	// RealmTenantID 是 username provider 的 realm；为空时使用 default realm。
	RealmTenantID meta.ID

	Profile map[string]string
	Meta    map[string]string
}

// MockConsumerUsernameLoginIdentityInput mock C 端账号开通输入，固定使用 default realm。
type MockConsumerUsernameLoginIdentityInput struct {
	Username string

	Profile map[string]string
	Meta    map[string]string
}

// WechatMiniLoginIdentityInput 微信小程序登录身份输入
type WechatMiniLoginIdentityInput struct {
	AppID   *string
	JsCode  *string
	OpenID  *string
	UnionID *string

	Profile map[string]string
	Meta    map[string]string
}

// OnboardingCredentialInput 凭据输入
type OnboardingCredentialInput struct {
	Password *PasswordCredentialInput
}

// PasswordCredentialInput 密码凭据输入
type PasswordCredentialInput struct {
	Plaintext string
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
	Credential *OnboardingCredential // nil 表示该登录身份不需要 IAM 保存长期 Credential

	// 状态
	IsNewUser          bool // 是否新建用户（true=新建，false=已存在）
	IsNewLoginIdentity bool // 是否新建登录身份（true=新建，false=已存在）
}

// OnboardingCredential 凭据
type OnboardingCredential struct {
	ID   meta.ID
	Type credDomain.CredentialType
}

// registrationRepositories 注册仓库
type registrationRepositories struct {
	Users           userDomain.Repository
	Credentials     credDomain.Repository
	LoginIdentities loginidentityDomain.Repository
}

// onboardingExecutionResult 开通执行结果
type onboardingExecutionResult struct {
	User          *UserResolveResult
	LoginIdentity *LoginIdentityEnsureResult
	Credential    *CredentialEnsureResult
}
