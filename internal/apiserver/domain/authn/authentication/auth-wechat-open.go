package authentication

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// WechatOpenCredential 认证凭据（微信开放平台登录所需的数据）
type WechatOpenCredential struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string
	AppID     string
	OpenID    string
	UnionID   string
}

// WechatOpenProofSpec 微信开放平台认证凭据规格
type WechatOpenProofSpec struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string
	AppID     string
	OpenID    string
	UnionID   string
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
	if spec.OpenID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat openid is required for wechat authentication")
	}
	return &WechatOpenCredential{
		TenantID:  spec.TenantID,
		RemoteIP:  spec.RemoteIP,
		UserAgent: spec.UserAgent,
		AppID:     spec.AppID,
		OpenID:    spec.OpenID,
		UnionID:   spec.UnionID,
	}, nil
}

// ================= 认证策略（执行认证的认证器） ========================

// OAuthWechatOpenAuthStrategy 微信开放平台认证策略
type OAuthWechatOpenAuthStrategy struct {
	credentialKind CredentialKind
	identityRepo   LoginIdentityRepository
}

// 实现认证策略接口
var _ AuthStrategy = (*OAuthWechatOpenAuthStrategy)(nil)

// NewOAuthWechatOpenAuthStrategyWithLoginIdentity 创建微信开放平台认证策略
func NewOAuthWechatOpenAuthStrategyWithLoginIdentity(
	identityRepo LoginIdentityRepository,
) *OAuthWechatOpenAuthStrategy {
	return &OAuthWechatOpenAuthStrategy{
		credentialKind: CredentialKindWechatOpen,
		identityRepo:   identityRepo,
	}
}

// Kind 返回认证策略类型
func (o *OAuthWechatOpenAuthStrategy) Kind() CredentialKind {
	return o.credentialKind
}

// Authenticate 执行微信开放平台认证
// 认证流程：
// 1. 按已验证的 openID 查找 LoginIdentity，必要时用 unionID 回退
// 2. 检查 LoginIdentity 状态
// 3. 返回认证判决
func (o *OAuthWechatOpenAuthStrategy) Authenticate(ctx context.Context, credential AuthCredential) (AuthDecision, error) {
	// 检查认证凭据类型
	wechatCred, ok := credential.(*WechatOpenCredential)
	if !ok {
		return AuthDecision{}, fmt.Errorf("wechat open strategy expects *WechatOpenCredential, got %T", credential)
	}

	identity := wechatOpenIdentity{openID: wechatCred.OpenID, unionID: wechatCred.UnionID}

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

// findWechatOpenIdentity 根据 openID 查找 LoginIdentity，必要时用 unionID 回退。
func (o *OAuthWechatOpenAuthStrategy) findWechatOpenIdentity(ctx context.Context, credential *WechatOpenCredential, identity wechatOpenIdentity) (*LoginIdentityLookup, error) {
	return findWechatIdentityByOpenIDThenUnionID(
		ctx,
		o.identityRepo,
		loginidentity.ProviderWechatOpen,
		credential.AppID,
		identity.openID,
		identity.unionID,
	)
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
