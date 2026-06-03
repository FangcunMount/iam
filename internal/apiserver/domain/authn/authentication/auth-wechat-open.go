package authentication

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// WechatOpenCredential 认证凭据（微信开放平台登录所需的数据）
type WechatOpenCredential struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string
	AppID     string
	AppSecret string
	Code      string
}

// WechatOpenProofSpec 微信开放平台认证凭据规格
type WechatOpenProofSpec struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string
	AppID     string
	AppSecret string
	Code      string
}

// CredentialKind 返回认证证明类型。
func (c *WechatOpenCredential) CredentialKind() CredentialKind {
	return CredentialKindWechatOpen
}

// NewWechatOpenCredential 构造微信开放平台认证凭据
func NewWechatOpenCredential(spec WechatOpenProofSpec) (AuthCredential, error) {
	if spec.AppID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat appid is required for wechat authentication")
	}
	if spec.AppSecret == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat appsecret is required for wechat authentication")
	}
	if spec.Code == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat code is required for wechat authentication")
	}
	return &WechatOpenCredential{
		TenantID:  spec.TenantID,
		RemoteIP:  spec.RemoteIP,
		UserAgent: spec.UserAgent,
		AppID:     spec.AppID,
		AppSecret: spec.AppSecret,
		Code:      spec.Code,
	}, nil
}

// ================= 认证策略（执行认证的认证器） ========================

// OAuthWechatOpenAuthStrategy 微信开放平台认证策略
type OAuthWechatOpenAuthStrategy struct {
	credentialKind CredentialKind
	identityRepo   LoginIdentityRepository
	idp            IdentityProvider
}

// 实现认证策略接口
var _ AuthStrategy = (*OAuthWechatOpenAuthStrategy)(nil)

// NewOAuthWechatOpenAuthStrategyWithLoginIdentity 创建微信开放平台认证策略
func NewOAuthWechatOpenAuthStrategyWithLoginIdentity(
	identityRepo LoginIdentityRepository,
	idp IdentityProvider,
) *OAuthWechatOpenAuthStrategy {
	return &OAuthWechatOpenAuthStrategy{
		credentialKind: CredentialKindWechatOpen,
		identityRepo:   identityRepo,
		idp:            idp,
	}
}

// Kind 返回认证策略类型
func (o *OAuthWechatOpenAuthStrategy) Kind() CredentialKind {
	return o.credentialKind
}

// Authenticate 执行微信开放平台认证
// 认证流程：
// 1. 调用微信 API 用 code 换取 openID/unionID
// 2. 按 openID 查找 LoginIdentity，必要时用 unionID 回退
// 3. 检查 LoginIdentity 状态
// 4. 返回认证判决
func (o *OAuthWechatOpenAuthStrategy) Authenticate(ctx context.Context, credential AuthCredential) (AuthDecision, error) {
	// 检查认证凭据类型
	wechatCred, ok := credential.(*WechatOpenCredential)
	if !ok {
		return AuthDecision{}, fmt.Errorf("wechat open strategy expects *WechatOpenCredential, got %T", credential)
	}

	// 与微信 IdP 交互，用 code 换取 openID/unionID
	identity, err := o.exchangeWechatOpenIdentity(ctx, wechatCred)
	if err != nil {
		return AuthDecision{}, err
	}

	// 根据openID查找凭据绑定
	lookup, err := o.findWechatOpenIdentity(ctx, wechatCred, identity)
	if err != nil {
		return AuthDecision{}, err
	}
	if lookup == nil || lookup.LoginIdentityID.IsZero() {
		return AuthDecision{
			OK:   false,
			Code: code.ErrNoBinding,
		}, nil
	}

	// 检查登录身份状态
	statusFailure, err := loginIdentityStatusFailureDecision(ctx, o.identityRepo, lookup.LoginIdentityID)
	if err != nil {
		return AuthDecision{}, err
	}
	if statusFailure != nil {
		return *statusFailure, nil
	}

	// 认证成功，构造Principal
	return o.buildWechatOpenSuccessDecision(ctx, wechatCred, identity, lookup.LoginIdentityID, lookup.UserID, meta.ZeroID), nil
}

// wechatOpenIdentity 微信开放平台身份
type wechatOpenIdentity struct {
	openID  string
	unionID string
}

// exchangeWechatOpenIdentity 与微信 IdP 交互，用 code 换取 openID/unionID。
func (o *OAuthWechatOpenAuthStrategy) exchangeWechatOpenIdentity(ctx context.Context, credential *WechatOpenCredential) (wechatOpenIdentity, error) {
	openID, unionID, err := o.idp.ExchangeWxOpenCode(ctx, credential.AppID, credential.AppSecret, credential.Code)
	if err != nil {
		return wechatOpenIdentity{}, fmt.Errorf("failed to exchange wx open code: %w", err)
	}

	return wechatOpenIdentity{
		openID:  openID,
		unionID: unionID,
	}, nil
}

// findWechatOpenIdentity 根据 openID 查找 LoginIdentity，必要时用 unionID 回退。
func (o *OAuthWechatOpenAuthStrategy) findWechatOpenIdentity(ctx context.Context, credential *WechatOpenCredential, identity wechatOpenIdentity) (*LoginIdentityLookup, error) {
	identifier := identity.unionID
	if identifier == "" {
		identifier = identity.openID
	}
	lookup, err := o.identityRepo.FindLoginIdentityByProviderKey(ctx, loginidentity.ProviderWechatOpen, credential.AppID, identifier)
	if err != nil || lookup != nil {
		return lookup, err
	}
	return o.identityRepo.FindLoginIdentityByGlobalIdentifier(ctx, loginidentity.ProviderWechatOpen, identity.unionID)
}

// buildWechatOpenSuccessDecision 认证成功，构造Principal
func (o *OAuthWechatOpenAuthStrategy) buildWechatOpenSuccessDecision(ctx context.Context, credential *WechatOpenCredential, identity wechatOpenIdentity, loginIdentityID meta.ID, userID meta.ID, credentialID meta.ID) AuthDecision {
	// 构造Principal
	principal := &Principal{
		LoginIdentityID: loginIdentityID,
		UserID:          userID,
		TenantID:        credential.TenantID,
		AuthMethod:      string(CredentialKindWechatOpen),
		Realm:           credential.AppID,
		AMR:             []string{string(AMRWxOpen)},
		Claims: map[string]any{
			"wx_openid":         identity.openID,
			"wx_unionid":        identity.unionID,
			"login_identity_id": loginIdentityID.String(),
			"auth_method":       string(CredentialKindWechatOpen),
			"realm":             credential.AppID,
			"auth_time":         ctx.Value("request_time"),
		},
	}

	// 返回认证决策
	return AuthDecision{
		OK:              true,
		Principal:       principal,
		LoginIdentityID: loginIdentityID,
		CredentialID:    credentialID,
	}
}
