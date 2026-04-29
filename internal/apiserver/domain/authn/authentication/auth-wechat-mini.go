package authentication

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ====================== 认证凭据（认证所需的数据） ========================

// WechatMinipCredential 认证凭据（微信小程序登录所需的数据）
type WechatMinipCredential struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string
	AppID     string
	AppSecret string
	Code      string
}

type WechatMiniProofSpec struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string
	AppID     string
	AppSecret string
	Code      string
}

// Scenario 返回认证场景
func (c *WechatMinipCredential) Scenario() Scenario {
	return AuthWxMinip
}

// NewWechatMiniCredential 构造微信小程序认证凭据
func NewWechatMiniCredential(spec WechatMiniProofSpec) (AuthCredential, error) {
	if spec.AppID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat appid is required for wechat authentication")
	}
	if spec.AppSecret == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat appsecret is required for wechat authentication")
	}
	if spec.Code == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat jscode is required for wechat authentication")
	}
	return &WechatMinipCredential{
		TenantID:  spec.TenantID,
		RemoteIP:  spec.RemoteIP,
		UserAgent: spec.UserAgent,
		AppID:     spec.AppID,
		AppSecret: spec.AppSecret,
		Code:      spec.Code,
	}, nil
}

// ================= 认证策略（执行认证的认证器） ========================

// OAuthWechatMinipAuthStrategy 微信小程序认证策略
type OAuthWechatMinipAuthStrategy struct {
	scenario    Scenario
	credRepo    CredentialRepository
	accountRepo AccountRepository
	idp         IdentityProvider
}

// 实现认证策略接口
var _ AuthStrategy = (*OAuthWechatMinipAuthStrategy)(nil)

// NewOAuthWechatMinipAuthStrategy 构造函数（注入依赖）
func NewOAuthWechatMinipAuthStrategy(
	credRepo CredentialRepository,
	accountRepo AccountRepository,
	idp IdentityProvider,
) *OAuthWechatMinipAuthStrategy {
	return &OAuthWechatMinipAuthStrategy{
		scenario:    AuthWxMinip,
		credRepo:    credRepo,
		accountRepo: accountRepo,
		idp:         idp,
	}
}

// Kind 返回认证策略类型
func (o *OAuthWechatMinipAuthStrategy) Kind() Scenario {
	return o.scenario
}

// Authenticate 执行微信小程序认证
// 认证流程：
// 1. 调用微信API用jsCode换取openID/unionID
// 2. 根据openID查找凭据绑定
// 3. 检查账户状态
// 4. 返回认证判决
func (o *OAuthWechatMinipAuthStrategy) Authenticate(ctx context.Context, credential AuthCredential) (AuthDecision, error) {
	wechatCred, ok := credential.(*WechatMinipCredential)
	if !ok {
		return AuthDecision{}, fmt.Errorf("wechat minip strategy expects *WechatMinipCredential, got %T", credential)
	}

	// Step 1: 与微信IdP交互，用jsCode换取openID

	openID, unionID, err := o.idp.ExchangeWxMinipCode(ctx, wechatCred.AppID, wechatCred.AppSecret, wechatCred.Code)
	if err != nil {
		// 系统异常或业务失败（code无效）
		return AuthDecision{
			OK:      false,
			ErrCode: ErrIDPExchangeFailed,
		}, fmt.Errorf("failed to exchange wx minip code: %w", err)
	}

	// Step 2: 根据openID查找凭据绑定（优先使用unionID，回退到openID）
	idpIdentifier := openID
	if unionID != "" {
		idpIdentifier = unionID
	}

	accountID, userID, credentialID, err := o.credRepo.FindOAuthCredential(ctx, string(AuthWxMinip), wechatCred.AppID, idpIdentifier)
	if err != nil {
		return AuthDecision{}, fmt.Errorf("failed to find wx minip credential: %w", err)
	}
	if credentialID.IsZero() {
		// 业务失败：微信账号未绑定
		return AuthDecision{
			OK:      false,
			ErrCode: ErrNoBinding,
		}, nil
	}

	// Step 3: 检查账户状态
	enabled, locked, err := o.accountRepo.GetAccountStatus(ctx, accountID)
	if err != nil {
		return AuthDecision{}, fmt.Errorf("failed to get account status: %w", err)
	}
	if !enabled {
		return AuthDecision{
			OK:      false,
			ErrCode: ErrDisabled,
		}, nil
	}
	if locked {
		return AuthDecision{
			OK:      false,
			ErrCode: ErrLocked,
		}, nil
	}

	// Step 4: 认证成功，构造Principal
	principal := &Principal{
		AccountID: accountID,
		UserID:    userID,
		TenantID:  wechatCred.TenantID,
		AMR:       []string{string(AMRWx)},
		Claims: map[string]any{
			"wx_openid":  openID,
			"wx_unionid": unionID,
			"auth_time":  ctx.Value("request_time"),
		},
	}

	return AuthDecision{
		OK:           true,
		Principal:    principal,
		CredentialID: credentialID,
	}, nil
}
