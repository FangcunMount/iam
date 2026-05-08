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

// PasswordCredential 认证凭据（用户名+密码）
type PasswordCredential struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string
	Username  string
	Password  string
}

type PasswordProofSpec struct {
	TenantID  meta.ID
	RemoteIP  string
	UserAgent string
	Username  string
	Password  string
}

// CredentialType 返回凭据类型。
func (c *PasswordCredential) CredentialType() credDomain.CredentialType {
	return credDomain.CredPassword
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
	credentialType credDomain.CredentialType
	credRepo       CredentialRepository
	accountRepo    AccountRepository
	hasher         PasswordHasher
}

// 实现认证策略接口
var _ AuthStrategy = (*PasswordAuthStrategy)(nil)

const passwordLoginOperaAccountType = "opera"

// NewPasswordAuthStrategy 构造函数（注入依赖）
func NewPasswordAuthStrategy(
	credRepo CredentialRepository,
	accountRepo AccountRepository,
	hasher PasswordHasher,
) *PasswordAuthStrategy {
	return &PasswordAuthStrategy{
		credentialType: credDomain.CredPassword,
		credRepo:       credRepo,
		accountRepo:    accountRepo,
		hasher:         hasher,
	}
}

// Kind 返回认证策略类型
func (p *PasswordAuthStrategy) Kind() credDomain.CredentialType {
	return p.credentialType
}

// Authenticate 执行用户名+密码认证
// 认证流程：
// 1. 根据用户名查找账户
// 2. 检查账户状态（是否锁定/禁用）
// 3. 查找密码凭据
// 4. 验证密码（带pepper）
// 5. 检查是否需要密码rehash（算法升级）
// 6. 返回认证判决
func (p *PasswordAuthStrategy) Authenticate(ctx context.Context, credential AuthCredential) (AuthDecision, error) {
	passwordCredential, ok := credential.(*PasswordCredential)
	if !ok {
		return AuthDecision{}, fmt.Errorf("password strategy expects *PasswordCredential, got %T", credential)
	}

	// Step 1: 根据用户名查找账户
	lookup, err := p.accountRepo.FindAccountByUsername(ctx, passwordCredential.TenantID, passwordCredential.Username)
	if err != nil {
		return AuthDecision{}, fmt.Errorf("failed to find account: %w", err)
	}
	if lookup == nil || lookup.AccountID.IsZero() {
		return AuthDecision{
			OK:   false,
			Code: code.ErrInvalidCredentials,
		}, nil
	}
	accountID, userID := lookup.AccountID, lookup.UserID

	principalTenant, ok := resolvePasswordPrincipalTenant(passwordCredential.TenantID, lookup)
	if !ok {
		return AuthDecision{
			OK:   false,
			Code: code.ErrInvalidCredentials,
		}, nil
	}

	// Step 2: 检查账户状态
	statusFailure, err := p.accountStatusFailure(ctx, accountID)
	if err != nil {
		return AuthDecision{}, err
	}
	if statusFailure != nil {
		return *statusFailure, nil
	}

	// Step 3: 查找密码凭据
	credentialID, storedHash, found, err := p.findPasswordCredential(ctx, accountID)
	if err != nil {
		return AuthDecision{}, err
	}
	if !found {
		return AuthDecision{
			OK:   false,
			Code: code.ErrInvalidCredentials,
		}, nil
	}

	// Step 4: 验证密码（加上全局pepper）
	plaintextWithPepper := passwordCredential.Password + p.hasher.Pepper()
	if !p.passwordMatches(storedHash, plaintextWithPepper) {
		// 密码错误（返回凭据ID用于失败次数统计）
		return AuthDecision{
			OK:           false,
			Code:         code.ErrInvalidCredentials,
			CredentialID: credentialID,
		}, nil
	}

	// Step 5: 检查是否需要密码rehash（例如算法参数升级）
	shouldRotate, newMaterial := p.rotationMaterial(storedHash, plaintextWithPepper)

	// Step 6: 认证成功，构造Principal
	principal := &Principal{
		AccountID: accountID,
		UserID:    userID,
		TenantID:  principalTenant,
		AMR:       []string{string(AMRPassword)},
		Claims: map[string]any{
			"auth_time": ctx.Value("request_time"),
		},
	}

	return AuthDecision{
		OK:           true,
		Principal:    principal,
		CredentialID: credentialID,
		ShouldRotate: shouldRotate,
		NewMaterial:  newMaterial,
	}, nil
}

// ================= 辅助方法 ========================

// 解析密码认证主体的租户ID
func resolvePasswordPrincipalTenant(requestTenantID meta.ID, lookup *UsernameLoginLookup) (meta.ID, bool) {
	if lookup.AccountType == passwordLoginOperaAccountType {
		if lookup.ScopedTenantID.IsZero() {
			return meta.ZeroID, false
		}
		if !requestTenantID.IsZero() && requestTenantID != lookup.ScopedTenantID {
			return meta.ZeroID, false
		}
		return lookup.ScopedTenantID, true
	}

	if !lookup.ScopedTenantID.IsZero() {
		return meta.ZeroID, false
	}
	return requestTenantID, true
}

// 检查账户状态是否失败
func (p *PasswordAuthStrategy) accountStatusFailure(ctx context.Context, accountID meta.ID) (*AuthDecision, error) {
	enabled, locked, err := p.accountRepo.GetAccountStatus(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account status: %w", err)
	}
	if !enabled {
		return &AuthDecision{
			OK:   false,
			Code: code.ErrCredentialDisabled,
		}, nil
	}
	if locked {
		return &AuthDecision{
			OK:   false,
			Code: code.ErrCredentialLocked,
		}, nil
	}

	return nil, nil
}

// findPasswordCredential 查找密码凭据
func (p *PasswordAuthStrategy) findPasswordCredential(ctx context.Context, accountID meta.ID) (meta.ID, string, bool, error) {
	credentialID, storedHash, err := p.credRepo.FindPasswordCredential(ctx, accountID)
	if err != nil {
		return meta.ZeroID, "", false, fmt.Errorf("failed to find password credential: %w", err)
	}
	if credentialID.IsZero() {
		return meta.ZeroID, "", false, nil
	}
	return credentialID, storedHash, true, nil
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
