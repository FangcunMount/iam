package authentication

import (
	"context"
	"fmt"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	credDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// ====================== 认证凭据（认证所需的数据） ========================

// PasswordProofSpec 密码认证凭据规范，用于构造 PasswordCredential 实例
type PasswordProofSpec struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string
	Username  string
	Password  string
}

// PasswordCredential 用户名+密码认证凭据
type PasswordCredential struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string
	Username  string
	Password  string
}

// 确保 PasswordCredential 实现了 AuthCredential 接口
var _ AuthCredential = (*PasswordCredential)(nil)

// CredentialKind 返回认证凭据类型
func (c *PasswordCredential) CredentialKind() CredentialKind {
	return CredentialKindPassword
}

// NewPasswordCredential 构造密码认证凭据
func NewPasswordCredential(spec PasswordProofSpec) (AuthCredential, error) {
	if spec.Username == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "username is required for password authentication")
	}
	if spec.Password == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "password is required for password authentication")
	}

	return &PasswordCredential{
		TenantID:  spec.TenantID,
		RemoteIP:  spec.RemoteIP,
		UserAgent: spec.UserAgent,
		Username:  spec.Username,
		Password:  spec.Password,
	}, nil
}

// ================= 认证策略（执行认证的认证器） ========================

// PasswordAuthStrategy 用户名+密码认证策略
type PasswordAuthStrategy struct {
	credentialKind CredentialKind
	credRepo       LoginIdentityCredentialRepository
	identityRepo   LoginIdentityRepository
	hasher         PasswordHasher
}

// 实现认证策略接口
var _ AuthStrategy = (*PasswordAuthStrategy)(nil)

func NewPasswordAuthStrategyWithLoginIdentity(
	credRepo LoginIdentityCredentialRepository,
	identityRepo LoginIdentityRepository,
	hasher PasswordHasher,
) *PasswordAuthStrategy {
	return &PasswordAuthStrategy{
		credentialKind: CredentialKindPassword,
		credRepo:       credRepo,
		identityRepo:   identityRepo,
		hasher:         hasher,
	}
}

// Kind 返回认证策略类型
func (p *PasswordAuthStrategy) Kind() CredentialKind {
	return p.credentialKind
}

// Authenticate 执行用户名+密码认证
// 认证流程：
// 1. 根据用户名查找 LoginIdentity
// 2. 检查 LoginIdentity 状态
// 3. 查找密码凭据
// 4. 验证密码（带pepper）
// 5. 检查是否需要密码rehash（算法升级）
// 6. 返回认证判决
func (p *PasswordAuthStrategy) Authenticate(ctx context.Context, credential AuthCredential) (AuthDecision, error) {
	// 断言认证凭据类型
	passwordCredential, ok := credential.(*PasswordCredential)
	if !ok {
		return AuthDecision{}, fmt.Errorf("password strategy expects *PasswordCredential, got %T", credential)
	}

	// 根据用户名查找登录身份
	lookup, err := p.identityRepo.FindUsernameIdentity(ctx, passwordCredential.TenantID, passwordCredential.Username)
	if err != nil {
		return AuthDecision{}, fmt.Errorf("failed to find login identity: %w", err)
	}
	// 如果登录身份不存在，则返回认证失败
	if lookup == nil || lookup.LoginIdentityID.IsZero() {
		return AuthDecision{
			OK:   false,
			Code: code.ErrInvalidCredentials,
		}, nil
	}
	// 获取登录身份ID和用户ID
	loginIdentityID, userID := lookup.LoginIdentityID, lookup.UserID

	// 检查登录身份状态
	statusFailure, err := loginIdentityStatusFailureDecision(ctx, p.identityRepo, loginIdentityID)
	if err != nil {
		return AuthDecision{}, err
	}
	if statusFailure != nil {
		return *statusFailure, nil
	}

	// 查找密码凭据
	passwordRecord, found, err := p.findPasswordCredential(ctx, loginIdentityID)
	if err != nil {
		return AuthDecision{}, err
	}
	// 如果密码凭据不存在，则返回认证失败
	if !found {
		return AuthDecision{
			OK:              false,
			Code:            code.ErrInvalidCredentials,
			LoginIdentityID: loginIdentityID,
		}, nil
	}
	// 获取密码凭据ID
	credentialID := passwordRecord.CredentialID
	// 检查密码凭据状态
	if passwordRecord.Status == credDomain.CredStatusDisabled {
		return AuthDecision{
			OK:              false,
			Code:            code.ErrCredentialDisabled,
			LoginIdentityID: loginIdentityID,
			CredentialID:    credentialID,
		}, nil
	}
	// 检查密码凭据是否被锁定
	if passwordRecord.LockedUntil != nil && time.Now().Before(*passwordRecord.LockedUntil) {
		return AuthDecision{
			OK:              false,
			Code:            code.ErrCredentialLocked,
			LoginIdentityID: loginIdentityID,
			CredentialID:    credentialID,
		}, nil
	}

	// 验证密码是否匹配
	plaintextWithPepper := passwordCredential.Password + p.hasher.Pepper()
	storedHash := passwordRecord.PasswordHash
	if !p.passwordMatches(storedHash, plaintextWithPepper) {
		return AuthDecision{
			OK:               false,
			Code:             code.ErrInvalidCredentials,
			LoginIdentityID:  loginIdentityID,
			CredentialID:     credentialID,
			CredentialEffect: CredentialEffectRecordFailure,
		}, nil
	}

	// 尝试生成升级后的密码 hash
	shouldRotate, newMaterial := p.rotationMaterial(storedHash, plaintextWithPepper)
	// 构造认证成功决策
	return p.buildPasswordSuccessDecision(ctx, passwordCredential, lookup, loginIdentityID, userID, credentialID, shouldRotate, newMaterial), nil
}

// ================= 辅助方法 ========================

// resolvePasswordPrincipalTenantFromIdentity 解析密码认证主体的租户ID
func resolvePasswordPrincipalTenantFromIdentity(requestTenantID meta.ID, lookup *LoginIdentityLookup) (meta.ID, bool) {
	if lookup == nil {
		return meta.ZeroID, false
	}
	if !lookup.ScopedTenantID.IsZero() {
		if !requestTenantID.IsZero() && requestTenantID != lookup.ScopedTenantID {
			return meta.ZeroID, false
		}
		return lookup.ScopedTenantID, true
	}
	if lookup.Provider == loginidentity.ProviderUsername && lookup.Realm != "" && lookup.Realm != loginidentity.RealmDefault {
		realmTenantID, err := meta.ParseID(lookup.Realm)
		if err == nil && !realmTenantID.IsZero() {
			if !requestTenantID.IsZero() && requestTenantID != realmTenantID {
				return meta.ZeroID, false
			}
			return realmTenantID, true
		}
	}
	return requestTenantID, true
}

func (p *PasswordAuthStrategy) findPasswordCredential(ctx context.Context, loginIdentityID meta.ID) (*PasswordCredentialLookup, bool, error) {
	record, err := p.credRepo.FindPasswordCredentialByLoginIdentity(ctx, loginIdentityID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to find password credential by login identity: %w", err)
	}
	if record == nil || record.CredentialID.IsZero() {
		return nil, false, nil
	}
	return record, true, nil
}

// 验证密码是否匹配
func (p *PasswordAuthStrategy) passwordMatches(storedHash string, plaintextWithPepper string) bool {
	return p.hasher.Verify(storedHash, plaintextWithPepper)
}

// rotationMaterial 尝试生成升级后的密码 hash。
// rehash 失败不应该把一次已经成功的登录变成认证失败。
func (p *PasswordAuthStrategy) rotationMaterial(storedHash string, plaintextWithPepper string) (bool, []byte) {
	if !p.hasher.NeedRehash(storedHash) {
		return false, nil
	}
	newHash, err := p.hasher.Hash(plaintextWithPepper)
	if err != nil {
		return false, nil
	}
	return true, []byte(newHash)
}

func (p *PasswordAuthStrategy) buildPasswordSuccessDecision(
	ctx context.Context,
	credential *PasswordCredential,
	lookup *LoginIdentityLookup,
	loginIdentityID meta.ID,
	userID meta.ID,
	credentialID meta.ID,
	shouldRotate bool,
	newMaterial []byte,
) AuthDecision {
	tenantID, _ := resolvePasswordPrincipalTenantFromIdentity(credential.TenantID, lookup)
	realm := lookup.Realm
	principal := &Principal{
		LoginIdentityID: loginIdentityID,
		UserID:          userID,
		TenantID:        tenantID,
		AuthMethod:      "password",
		Realm:           realm,
		AMR:             []string{string(AMRPassword)},
		Claims: map[string]any{
			"login_identity_id": loginIdentityID.String(),
			"auth_method":       "password",
			"realm":             realm,
			"auth_time":         ctx.Value("request_time"),
		},
	}

	return AuthDecision{
		OK:               true,
		Principal:        principal,
		LoginIdentityID:  loginIdentityID,
		CredentialID:     credentialID,
		CredentialEffect: CredentialEffectRecordSuccess,
		ShouldRotate:     shouldRotate,
		NewMaterial:      newMaterial,
	}
}
