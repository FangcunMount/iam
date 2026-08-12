package authentication

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
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

// CredentialKind 返回认证证明类型。
func (c *WechatMinipCredential) CredentialKind() CredentialKind {
	return CredentialKindWechatMinip
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
	credentialKind CredentialKind
	identityRepo   LoginIdentityRepository
	idp            IdentityProvider
}

// 实现认证策略接口
var _ AuthStrategy = (*OAuthWechatMinipAuthStrategy)(nil)

func NewOAuthWechatMinipAuthStrategyWithLoginIdentity(
	identityRepo LoginIdentityRepository,
	idp IdentityProvider,
) *OAuthWechatMinipAuthStrategy {
	return &OAuthWechatMinipAuthStrategy{
		credentialKind: CredentialKindWechatMinip,
		identityRepo:   identityRepo,
		idp:            idp,
	}
}

// Kind 返回认证策略类型
func (o *OAuthWechatMinipAuthStrategy) Kind() CredentialKind {
	return o.credentialKind
}

// Authenticate 执行微信小程序认证
// 认证流程：
// 1. 调用微信API用jsCode换取openID/unionID
// 2. 根据openID查找凭据绑定
// 3. 检查 LoginIdentity 状态
// 4. 返回认证判决
func (o *OAuthWechatMinipAuthStrategy) Authenticate(ctx context.Context, credential AuthCredential) (AuthDecision, error) {
	wechatCred, ok := credential.(*WechatMinipCredential)
	if !ok {
		return AuthDecision{}, fmt.Errorf("wechat minip strategy expects *WechatMinipCredential, got %T", credential)
	}
	identity, err := o.exchangeWechatMinipIdentity(ctx, wechatCred)
	if err != nil {
		return AuthDecision{}, err
	}

	lookup, err := o.findWechatMinipIdentity(ctx, wechatCred, identity)
	if err != nil {
		return AuthDecision{}, err
	}
	if lookup == nil || lookup.LoginIdentityID.IsZero() {
		return AuthDecision{
			OK:   false,
			Code: code.ErrNoBinding,
		}, nil
	}

	statusFailure, err := loginIdentityStatusFailureDecision(ctx, o.identityRepo, lookup.LoginIdentityID)
	if err != nil {
		return AuthDecision{}, err
	}
	if statusFailure != nil {
		return *statusFailure, nil
	}

	return o.buildWechatMinipSuccessDecision(ctx, wechatCred, identity, lookup.LoginIdentityID, lookup.UserID, meta.ZeroID), nil
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

func (o *OAuthWechatMinipAuthStrategy) findWechatMinipIdentity(
	ctx context.Context,
	credential *WechatMinipCredential,
	identity wechatMinipIdentity,
) (*LoginIdentityLookup, error) {
	return findWechatIdentityByOpenIDThenUnionID(
		ctx,
		o.identityRepo,
		loginidentity.ProviderWechatMinip,
		credential.AppID,
		identity.openID,
		identity.unionID,
	)
}

// buildWechatMinipSuccessDecision 认证成功，构造Principal
func (o *OAuthWechatMinipAuthStrategy) buildWechatMinipSuccessDecision(
	ctx context.Context,
	credential *WechatMinipCredential,
	identity wechatMinipIdentity,
	loginIdentityID meta.ID,
	userID meta.ID,
	credentialID meta.ID,
) AuthDecision {
	principal := &Principal{
		LoginIdentityID: loginIdentityID,
		UserID:          userID,
		TenantID:        credential.TenantID,
		AuthMethod:      "wechat_minip",
		Realm:           credential.AppID,
		AMR:             []string{string(AMRWx)},
		Claims: map[string]any{
			"wx_openid":         identity.openID,
			"wx_unionid":        identity.unionID,
			"login_identity_id": loginIdentityID.String(),
			"auth_method":       "wechat_minip",
			"realm":             credential.AppID,
			"auth_time":         ctx.Value("request_time"),
		},
	}

	return AuthDecision{
		OK:              true,
		Principal:       principal,
		LoginIdentityID: loginIdentityID,
		CredentialID:    credentialID,
	}
}
