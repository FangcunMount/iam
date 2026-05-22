package linking

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	idpPort "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ==========================================================================
// ================== Interface (Driving Ports & DTOs) ======================
// ==========================================================================
//
// 绑定入口通过 LinkRequest.Input 多态 prepare（见 link_phone.go、link_wechat.go、link_wecom.go）。

// Linker 管理已认证用户的登录身份绑定。
//
// 手机号绑定发码由 transport 通过 challenge 发送用例完成，不在本接口暴露。
type Linker interface {
	// List 列出当前用户登录身份。
	List(ctx context.Context, userID meta.ID) ([]LoginIdentityView, error)
	// Link 绑定登录身份。
	Link(ctx context.Context, req LinkRequest) (*LinkResult, error)
	// Unlink 解绑登录身份。
	Unlink(ctx context.Context, cmd UnlinkCommand) error
}

// PhoneLinkChallengeVerifier 是手机号绑定用例依赖的短期挑战消费端口。
type PhoneLinkChallengeVerifier interface {
	VerifyAndConsumePhoneLinkOTP(ctx context.Context, phoneE164, otp string) bool
}

// Dependencies 是登录身份绑定应用服务依赖。
type Dependencies struct {
	LoginIdentities  loginidentity.Repository
	PhoneLinkOTP     PhoneLinkChallengeVerifier
	IDP              authentication.IdentityProvider
	WechatApps       idpPort.Repository
	SecretVault      idpPort.SecretVault
	WecomAgentID     string
	RecentAuthWindow time.Duration
	Now              func() time.Time
}

// LinkRequest 统一绑定请求；Input 决定 prepare 变体。
type LinkRequest struct {
	UserID meta.ID                // 用户 ID。
	Input  LinkLoginIdentityInput // 登录身份输入。
}

// LinkLoginIdentityInput 各绑定入口实现本接口，在 prepare 阶段解析外部凭证并产出 ProviderKey。
type LinkLoginIdentityInput interface {
	// PrepareLink 准备登录身份。
	prepareLink(context.Context, linkPrepareDeps, meta.ID) (preparedLink, error)
}

// LinkResult 是绑定登录身份后的结果。
type LinkResult struct {
	Identity *loginidentity.LoginIdentity // 登录身份。
	Reused   bool                         // 是否复用已存在的登录身份。
}

// preparedLink 是 ensure 阶段共享的准备结果（包内）。
type preparedLink struct {
	key                     loginidentity.ProviderKey
	build                   func() (*loginidentity.LoginIdentity, error)
	requireGlobalUniqueness bool
}

// LoginIdentityView 是当前用户已绑定登录身份的只读视图。
type LoginIdentityView struct {
	//----- 基础信息 -----//
	ID       meta.ID                // 登录身份 ID。
	UserID   meta.ID                // 用户 ID。
	Provider loginidentity.Provider // 提供者。

	//----- 身份信息 -----//
	Realm            string // 域。
	Identifier       string // 标识。
	GlobalIdentifier string // 全局标识。

	//----- 状态信息 -----//
	Status     loginidentity.Status // 状态。
	VerifiedAt *time.Time           // 验证时间。
	LinkedAt   time.Time            // 绑定时间。
}

// LinkPhoneInput 手机号绑定输入。
type LinkPhoneInput struct {
	Phone   string
	OTPCode string
}

// LinkWechatMiniInput 微信小程序绑定输入。
type LinkWechatMiniInput struct {
	AppID string // 微信小程序应用 ID。
	Code  string // 微信小程序认证码。
}

// LinkWecomInput 企业微信绑定输入。
type LinkWecomInput struct {
	CorpID string // 企业微信 Corp ID。
	Code   string // 企业微信认证码。
}

// UnlinkCommand 是解绑登录身份命令。
type UnlinkCommand struct {
	UserID                 meta.ID // 用户 ID。
	LoginIdentityID        meta.ID // 登录身份 ID。
	CurrentLoginIdentityID meta.ID // 当前登录身份 ID。
	// AuthenticatedAt 由 transport 从已验 access token 的 auth_time（或等价 claim）填入，用于敏感解绑的近期认证窗口。
	// 当前无独立 application 再认证 API；调用方须先完成 token 验票或等价门禁，勿伪造时间戳。
	AuthenticatedAt *time.Time
}
