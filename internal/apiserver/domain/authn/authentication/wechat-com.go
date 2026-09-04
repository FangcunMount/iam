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

// WecomProofSpec 企业微信认证凭据规范，用于构造 WecomCredential 实例
type WecomProofSpec struct {
	TenantID   meta.ID
	RemoteIP   string
	UserAgent  string
	CorpID     string
	UserID     string
	OpenUserID string
	State      string
}

// WecomCredential 企业微信认证凭据
type WecomCredential struct {
	TenantID   meta.ID
	RemoteIP   string
	UserAgent  string
	CorpID     string
	UserID     string
	OpenUserID string
	State      string
}

// 确保 WecomCredential 实现了 AuthCredential 接口
var _ AuthCredential = (*WecomCredential)(nil)

// CredentialKind 返回认证凭据类型
func (c *WecomCredential) CredentialKind() CredentialKind {
	return CredentialKindWecom
}

// NewWecomCredential 创建 WecomCredential 实例
func NewWecomCredential(spec WecomProofSpec) (AuthCredential, error) {
	if spec.CorpID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wecom corpid is required for wecom authentication")
	}
	if spec.UserID == "" && spec.OpenUserID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wecom userid or open_userid is required for wecom authentication")
	}
	return &WecomCredential{
		TenantID:   spec.TenantID,
		RemoteIP:   spec.RemoteIP,
		UserAgent:  spec.UserAgent,
		CorpID:     spec.CorpID,
		UserID:     spec.UserID,
		OpenUserID: spec.OpenUserID,
		State:      spec.State,
	}, nil
}

// ================= 认证策略（执行认证的认证器） ========================

// OAuthWeChatComAuthStrategy 企业微信认证策略
type OAuthWeChatComAuthStrategy struct {
	credentialKind CredentialKind
	identityRepo   LoginIdentityRepository
}

// 实现认证策略接口
var _ AuthStrategy = (*OAuthWeChatComAuthStrategy)(nil)

func NewOAuthWeChatComAuthStrategyWithLoginIdentity(
	identityRepo LoginIdentityRepository,
) *OAuthWeChatComAuthStrategy {
	return &OAuthWeChatComAuthStrategy{
		credentialKind: CredentialKindWecom,
		identityRepo:   identityRepo,
	}
}

// Kind 返回认证策略类型
func (o *OAuthWeChatComAuthStrategy) Kind() CredentialKind {
	return o.credentialKind
}

// Authenticate 执行企业微信认证
// 认证流程：
// 1. 根据已验证的 UserID/OpenUserID 查找凭据绑定
// 2. 检查 LoginIdentity 状态
// 3. 返回认证判决
func (o *OAuthWeChatComAuthStrategy) Authenticate(ctx context.Context, credential AuthCredential) (AuthDecision, error) {
	// 断言认证凭据类型
	wecomCred, ok := credential.(*WecomCredential)
	if !ok {
		return AuthDecision{}, fmt.Errorf("wecom strategy expects *WecomCredential, got %T", credential)
	}

	// 根据已验证的 UserID/OpenUserID 查找登录身份
	identity := wecomIdentity{openUserID: wecomCred.OpenUserID, userID: wecomCred.UserID}
	lookup, err := o.findWecomIdentity(ctx, wecomCred, identity)
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
	return o.buildWecomSuccessDecision(ctx, wecomCred, identity, lookup.LoginIdentityID, lookup.UserID, meta.ZeroID), nil
}

// wecomIdentity 企业微信身份
type wecomIdentity struct {
	openUserID string
	userID     string
}

// findWecomIdentity 根据 UserID/OpenUserID 查找登录身份
func (o *OAuthWeChatComAuthStrategy) findWecomIdentity(
	ctx context.Context,
	credential *WecomCredential,
	identity wecomIdentity,
) (*LoginIdentityLookup, error) {
	// 根据 UserID/OpenUserID 查找登录身份
	identifier := identity.userID
	if identifier == "" {
		identifier = identity.openUserID
	}
	lookup, err := o.identityRepo.FindLoginIdentityByProviderKey(ctx, loginidentity.ProviderWecom, credential.CorpID, identifier)
	if err != nil || lookup != nil || identity.openUserID == "" || identifier == identity.openUserID {
		return lookup, err
	}
	return o.identityRepo.FindLoginIdentityByProviderKey(ctx, loginidentity.ProviderWecom, credential.CorpID, identity.openUserID)
}

// buildWecomSuccessDecision 认证成功，构造Principal
func (o *OAuthWeChatComAuthStrategy) buildWecomSuccessDecision(
	ctx context.Context,
	credential *WecomCredential,
	identity wecomIdentity,
	loginIdentityID meta.ID,
	userID meta.ID,
	credentialID meta.ID,
) AuthDecision {
	principal := &Principal{
		LoginIdentityID: loginIdentityID,
		UserID:          userID,
		TenantID:        credential.TenantID,
	}
	principal.ApplyAuthContext(NewAuthenticationContext(MethodWecom, credential.CorpID, []AMR{AMRWecom}, time.Now().UTC()))

	return AuthDecision{
		OK:              true,
		Principal:       principal,
		LoginIdentityID: loginIdentityID,
		CredentialID:    credentialID,
	}
}
