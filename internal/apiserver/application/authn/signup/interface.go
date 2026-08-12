package signup

import (
	"context"

	credDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/credential"
	userDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// ==========================================================================
// ================== Interface (Driving Ports & DTOs) ======================
// ==========================================================================
//
// 对外契约：transport / assembler 仅依赖本文件类型与 SignupService。
// 用例实现见 service.go；入口变体实现见 login_identity.go、wechat_signup.go、mock_consumer_ensure.go。

// SignupService 登录身份开通应用服务（与 /authn/signups 路由一致）。
type SignupService interface {
	// SignUp 完成：Prepare → ResolveUser → EnsureLoginIdentity → EnsureCredential。
	SignUp(ctx context.Context, req SignupRequest) (*SignupResult, error)
}

// SignupLoginIdentityInput 登录身份输入；各开通入口实现本接口产出 ProviderKey。
type SignupLoginIdentityInput interface {
	prepareSignupLoginIdentity(context.Context, loginIdentityPrepareDeps, SignupUserInput) (preparedLoginIdentity, error)
}

// SignupRequest 统一登录身份开通请求。
type SignupRequest struct {
	User          SignupUserInput
	LoginIdentity SignupLoginIdentityInput
	Credential    *SignupCredentialInput
}

// SignupUserInput 用户输入（Name、Phone、Email 由领域校验）。
type SignupUserInput struct {
	Name  string
	Phone meta.Phone
	Email meta.Email
}

// UsernameLoginIdentityInput 租户内用户名+密码开通输入。
type UsernameLoginIdentityInput struct {
	Username string
	// RealmTenantID 是 username provider 的 realm；为空时使用 default realm。
	RealmTenantID meta.ID
	Profile       map[string]string
	Meta          map[string]string
}

// WechatMiniLoginIdentityInput 微信小程序 /signups/wechat-miniprogram 输入。
type WechatMiniLoginIdentityInput struct {
	AppID   *string
	JsCode  *string
	OpenID  *string
	UnionID *string
	Profile map[string]string
	Meta    map[string]string
}

// MockConsumerUsernameLoginIdentityInput 内部 mock C 端 ensure 输入，固定 default realm。
type MockConsumerUsernameLoginIdentityInput struct {
	Username string
	Profile  map[string]string
	Meta     map[string]string
}

// SignupCredentialInput 凭据输入。
type SignupCredentialInput struct {
	Password *PasswordCredentialInput
}

// PasswordCredentialInput 密码凭据输入。
type PasswordCredentialInput struct {
	Plaintext string
}

// SignupResult 登录身份开通结果。
type SignupResult struct {
	UserID             meta.ID
	UserName           string
	Phone              meta.Phone
	Email              meta.Email
	UserStatus         userDomain.Status
	LoginIdentityID    meta.ID
	Credential         *SignupCredential
	IsNewUser          bool
	IsNewLoginIdentity bool
}

// SignupCredential 凭据摘要。
type SignupCredential struct {
	ID   meta.ID
	Type credDomain.CredentialType
}
