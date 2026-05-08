package authentication

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
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

// CredentialType 返回凭据类型。
func (c *WechatMinipCredential) CredentialType() credDomain.CredentialType {
	return credDomain.CredOAuthWxMinip
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
	credentialType credDomain.CredentialType
	credRepo       CredentialRepository
	accountRepo    AccountRepository
	idp            IdentityProvider
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
		credentialType: credDomain.CredOAuthWxMinip,
		credRepo:       credRepo,
		accountRepo:    accountRepo,
		idp:            idp,
	}
}

// Kind 返回认证策略类型
func (o *OAuthWechatMinipAuthStrategy) Kind() credDomain.CredentialType {
	return o.credentialType
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
	identity, err := o.exchangeWechatMinipIdentity(ctx, wechatCred)
	if err != nil {
		return AuthDecision{}, err
	}

	// Step 2: 根据openID查找凭据绑定（优先使用unionID，回退到openID）
	accountID, userID, credentialID, found, err := o.findWechatMinipCredential(ctx, wechatCred, identity)
	if err != nil {
		return AuthDecision{}, err
	}
	if !found {
		return AuthDecision{
			OK:   false,
			Code: code.ErrNoBinding,
		}, nil
	}

	// Step 3: 检查账户状态
	statusFailure, err := accountStatusFailureDecision(ctx, o.accountRepo, accountID)
	if err != nil {
		return AuthDecision{}, err
	}
	if statusFailure != nil {
		return *statusFailure, nil
	}

	// Step 4: 认证成功，构造Principal
	return o.buildWechatMinipSuccessDecision(ctx, wechatCred, identity, accountID, userID, credentialID), nil
}

// wechatMinipIdentity 微信小程序身份
type wechatMinipIdentity struct {
	openID  string
	unionID string
}

// exchangeWechatMinipIdentity 与微信IdP交互，用jsCode换取openID
func (o *OAuthWechatMinipAuthStrategy) exchangeWechatMinipIdentity(ctx context.Context, credential *WechatMinipCredential) (wechatMinipIdentity, error) {
	openID, unionID, err := o.idp.ExchangeWxMinipCode(ctx, credential.AppID, credential.AppSecret, credential.Code)
	if err != nil {
		return wechatMinipIdentity{}, fmt.Errorf("failed to exchange wx minip code: %w", err)
	}
	return wechatMinipIdentity{
		openID:  openID,
		unionID: unionID,
	}, nil
}

// findWechatMinipCredential 根据openID查找凭据绑定
func (o *OAuthWechatMinipAuthStrategy) findWechatMinipCredential(
	ctx context.Context,
	credential *WechatMinipCredential,
	identity wechatMinipIdentity,
) (meta.ID, meta.ID, meta.ID, bool, error) {
	accountID, userID, credentialID, err := o.credRepo.FindOAuthCredential(
		ctx,
		string(credDomain.CredOAuthWxMinip),
		credential.AppID,
		wechatMinipIDPIdentifier(identity),
	)
	if err != nil {
		return meta.ZeroID, meta.ZeroID, meta.ZeroID, false, fmt.Errorf("failed to find wx minip credential: %w", err)
	}
	if credentialID.IsZero() {
		return meta.ZeroID, meta.ZeroID, meta.ZeroID, false, nil
	}
	return accountID, userID, credentialID, true, nil
}

// wechatMinipIDPIdentifier 微信小程序IDP标识符
func wechatMinipIDPIdentifier(identity wechatMinipIdentity) string {
	if identity.unionID != "" {
		return identity.unionID
	}
	return identity.openID
}

// buildWechatMinipSuccessDecision 认证成功，构造Principal
func (o *OAuthWechatMinipAuthStrategy) buildWechatMinipSuccessDecision(
	ctx context.Context,
	credential *WechatMinipCredential,
	identity wechatMinipIdentity,
	accountID meta.ID,
	userID meta.ID,
	credentialID meta.ID,
) AuthDecision {
	principal := &Principal{
		AccountID: accountID,
		UserID:    userID,
		TenantID:  credential.TenantID,
		AMR:       []string{string(AMRWx)},
		Claims: map[string]any{
			"wx_openid":  identity.openID,
			"wx_unionid": identity.unionID,
			"auth_time":  ctx.Value("request_time"),
		},
	}

	return AuthDecision{
		OK:           true,
		Principal:    principal,
		AccountID:    accountID,
		CredentialID: credentialID,
	}
}
