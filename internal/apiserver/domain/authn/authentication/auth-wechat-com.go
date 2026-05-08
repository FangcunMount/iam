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

// WecomCredential 认证凭据（企业微信登录所需的数据）
type WecomCredential struct {
	TenantID   meta.ID
	RemoteIP   string
	UserAgent  string
	CorpID     string
	AgentID    string
	CorpSecret string
	Code       string
	State      string
}

type WecomProofSpec struct {
	TenantID   meta.ID
	RemoteIP   string
	UserAgent  string
	CorpID     string
	AgentID    string
	CorpSecret string
	Code       string
	State      string
}

// CredentialType 返回凭据类型。
func (c *WecomCredential) CredentialType() credDomain.CredentialType {
	return credDomain.CredOAuthWecom
}

// NewWecomCredential 构造企业微信认证凭据
func NewWecomCredential(spec WecomProofSpec) (AuthCredential, error) {
	if spec.CorpID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wecom corpid is required for wecom authentication")
	}
	if spec.AgentID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wecom agentid is required for wecom authentication")
	}
	if spec.CorpSecret == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wecom corpsecret is required for wecom authentication")
	}
	if spec.Code == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wecom code is required for wecom authentication")
	}
	return &WecomCredential{
		TenantID:   spec.TenantID,
		RemoteIP:   spec.RemoteIP,
		UserAgent:  spec.UserAgent,
		CorpID:     spec.CorpID,
		AgentID:    spec.AgentID,
		CorpSecret: spec.CorpSecret,
		Code:       spec.Code,
		State:      spec.State,
	}, nil
}

// ================= 认证策略（执行认证的认证器） ========================

// OAuthWeChatComAuthStrategy 企业微信认证策略
type OAuthWeChatComAuthStrategy struct {
	credentialType credDomain.CredentialType
	credRepo       CredentialRepository
	accountRepo    AccountRepository
	idp            IdentityProvider
}

// 实现认证策略接口
var _ AuthStrategy = (*OAuthWeChatComAuthStrategy)(nil)

// NewOAuthWeChatComAuthStrategy 构造函数（注入依赖）
func NewOAuthWeChatComAuthStrategy(
	credRepo CredentialRepository,
	accountRepo AccountRepository,
	idp IdentityProvider,
) *OAuthWeChatComAuthStrategy {
	return &OAuthWeChatComAuthStrategy{
		credentialType: credDomain.CredOAuthWecom,
		credRepo:       credRepo,
		accountRepo:    accountRepo,
		idp:            idp,
	}
}

// Kind 返回认证策略类型
func (o *OAuthWeChatComAuthStrategy) Kind() credDomain.CredentialType {
	return o.credentialType
}

// Authenticate 执行企业微信认证
// 认证流程：
// 1. 调用企业微信API用code换取用户信息
// 2. 根据UserID查找凭据绑定
// 3. 检查账户状态
// 4. 返回认证判决
func (o *OAuthWeChatComAuthStrategy) Authenticate(ctx context.Context, credential AuthCredential) (AuthDecision, error) {
	wecomCred, ok := credential.(*WecomCredential)
	if !ok {
		return AuthDecision{}, fmt.Errorf("wecom strategy expects *WecomCredential, got %T", credential)
	}

	// Step 1: 与企业微信IdP交互，用code换取用户信息
	identity, err := o.exchangeWecomIdentity(ctx, wecomCred)
	if err != nil {
		return AuthDecision{}, err
	}

	// Step 2: 根据UserID查找凭据绑定（优先使用UserID，回退到OpenUserID）
	accountID, userID, credentialID, found, err := o.findWecomCredential(ctx, wecomCred, identity)
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
	return o.buildWecomSuccessDecision(ctx, wecomCred, identity, accountID, userID, credentialID), nil
}

// wecomIdentity 企业微信身份
type wecomIdentity struct {
	openUserID string
	userID     string
}

// exchangeWecomIdentity 与企业微信IdP交互，用code换取用户信息
func (o *OAuthWeChatComAuthStrategy) exchangeWecomIdentity(ctx context.Context, credential *WecomCredential) (wecomIdentity, error) {
	openUserID, userID, err := o.idp.ExchangeWecomCode(ctx, credential.CorpID, credential.AgentID, credential.CorpSecret, credential.Code)
	if err != nil {
		return wecomIdentity{}, fmt.Errorf("failed to exchange wecom code: %w", err)
	}
	return wecomIdentity{
		openUserID: openUserID,
		userID:     userID,
	}, nil
}

// findWecomCredential 根据UserID查找凭据绑定
func (o *OAuthWeChatComAuthStrategy) findWecomCredential(
	ctx context.Context,
	credential *WecomCredential,
	identity wecomIdentity,
) (meta.ID, meta.ID, meta.ID, bool, error) {
	accountID, userID, credentialID, err := o.credRepo.FindOAuthCredential(
		ctx,
		"wecom",
		credential.CorpID,
		wecomIDPIdentifier(identity),
	)
	if err != nil {
		return meta.ZeroID, meta.ZeroID, meta.ZeroID, false, fmt.Errorf("failed to find wecom credential: %w", err)
	}
	if credentialID.IsZero() {
		return meta.ZeroID, meta.ZeroID, meta.ZeroID, false, nil
	}
	return accountID, userID, credentialID, true, nil
}

// wecomIDPIdentifier 企业微信IDP标识符
func wecomIDPIdentifier(identity wecomIdentity) string {
	if identity.userID != "" {
		return identity.userID
	}
	return identity.openUserID
}

// buildWecomSuccessDecision 认证成功，构造Principal
func (o *OAuthWeChatComAuthStrategy) buildWecomSuccessDecision(
	ctx context.Context,
	credential *WecomCredential,
	identity wecomIdentity,
	accountID meta.ID,
	userID meta.ID,
	credentialID meta.ID,
) AuthDecision {
	principal := &Principal{
		AccountID: accountID,
		UserID:    userID,
		TenantID:  credential.TenantID,
		AMR:       []string{string(AMRWecom)},
		Claims: map[string]any{
			"wecom_corp_id":      credential.CorpID,
			"wecom_state":        credential.State,
			"wecom_user_id":      identity.userID,
			"wecom_open_user_id": identity.openUserID,
			"auth_time":          ctx.Value("request_time"),
		},
	}

	return AuthDecision{
		OK:           true,
		Principal:    principal,
		AccountID:    accountID,
		CredentialID: credentialID,
	}
}
