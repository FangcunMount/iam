package authentication

import (
	"context"
	"fmt"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// ====================== 认证凭据（认证所需的数据） ========================

// WechatMiniProofSpec 微信小程序认证凭据规范
type WechatMiniProofSpec struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string
	AppID     string
	OpenID    string
	UnionID   string
}

// WechatMinipCredential 微信小程序认证凭据
type WechatMinipCredential struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string
	AppID     string
	OpenID    string
	UnionID   string
}

// 确保 WechatMinipCredential 实现了 AuthCredential 接口
var _ AuthCredential = (*WechatMinipCredential)(nil)

// CredentialKind 返回认证凭据类型
func (c *WechatMinipCredential) CredentialKind() CredentialKind {
	return CredentialKindWechatMinip
}

// NewWechatMiniCredential 创建 WechatMinipCredential 实例
func NewWechatMiniCredential(spec WechatMiniProofSpec) (AuthCredential, error) {
	if spec.AppID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat appid is required for wechat authentication")
	}
	if spec.OpenID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat openid is required for wechat authentication")
	}
	return &WechatMinipCredential{
		TenantID:  spec.TenantID,
		RemoteIP:  spec.RemoteIP,
		UserAgent: spec.UserAgent,
		AppID:     spec.AppID,
		OpenID:    spec.OpenID,
		UnionID:   spec.UnionID,
	}, nil
}

// ================= 认证策略（执行认证的认证器） ========================

// OAuthWechatMinipAuthStrategy 微信小程序认证策略
type OAuthWechatMinipAuthStrategy struct {
	credentialKind CredentialKind
	identityRepo   LoginIdentityRepository
}

// 实现认证策略接口
var _ AuthStrategy = (*OAuthWechatMinipAuthStrategy)(nil)

func NewOAuthWechatMinipAuthStrategyWithLoginIdentity(
	identityRepo LoginIdentityRepository,
) *OAuthWechatMinipAuthStrategy {
	return &OAuthWechatMinipAuthStrategy{
		credentialKind: CredentialKindWechatMinip,
		identityRepo:   identityRepo,
	}
}

// Kind 返回认证策略类型
func (o *OAuthWechatMinipAuthStrategy) Kind() CredentialKind {
	return o.credentialKind
}

// Authenticate 执行微信小程序认证
// 认证流程：
// 1. 根据已验证的 openID/unionID 查找凭据绑定
// 2. 检查 LoginIdentity 状态
// 3. 返回认证判决
func (o *OAuthWechatMinipAuthStrategy) Authenticate(ctx context.Context, credential AuthCredential) (AuthDecision, error) {
	// 断言认证凭据类型
	wechatCred, ok := credential.(*WechatMinipCredential)
	if !ok {
		return AuthDecision{}, fmt.Errorf("wechat minip strategy expects *WechatMinipCredential, got %T", credential)
	}
	identity := wechatMinipIdentity{openID: wechatCred.OpenID, unionID: wechatCred.UnionID}

	// 根据已验证的 openID/unionID 查找登录身份
	lookup, err := o.findWechatMinipIdentity(ctx, wechatCred, identity)
	if err != nil {
		return AuthDecision{}, err
	}
	// 如果登录身份不存在，则返回认证失败
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
	// 如果登录身份状态为失败，则返回认证失败
	if statusFailure != nil {
		return *statusFailure, nil
	}

	// 构造认证成功决策
	return o.buildWechatMinipSuccessDecision(ctx, wechatCred, identity, lookup.LoginIdentityID, lookup.UserID, meta.ZeroID), nil
}

// wechatMinipIdentity 微信小程序身份
type wechatMinipIdentity struct {
	openID  string
	unionID string
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
	}
	principal.ApplyAuthContext(NewAuthenticationContext(MethodWechatMinip, credential.AppID, []AMR{AMRWx}, time.Now().UTC()))

	return AuthDecision{
		OK:              true,
		Principal:       principal,
		LoginIdentityID: loginIdentityID,
		CredentialID:    credentialID,
	}
}
