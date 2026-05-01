package authentication

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
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
	l := logger.L(ctx)

	passwordCredential, ok := credential.(*PasswordCredential)
	if !ok {
		return AuthDecision{}, fmt.Errorf("password strategy expects *PasswordCredential, got %T", credential)
	}

	l.Debugw("密码认证：步骤1 - 根据用户名查找账户",
		"credential_type", string(credDomain.CredPassword),
		"username", passwordCredential.Username,
	)

	// Step 1: 根据用户名查找账户
	lookup, err := p.accountRepo.FindAccountByUsername(ctx, passwordCredential.TenantID, passwordCredential.Username)
	if err != nil {
		// 系统异常（如数据库错误）
		l.Errorw("查询账户失败",
			"credential_type", string(credDomain.CredPassword),
			"username", passwordCredential.Username,
			"error", err.Error(),
		)
		return AuthDecision{}, fmt.Errorf("failed to find account: %w", err)
	}
	if lookup == nil || lookup.AccountID.IsZero() {
		// 业务失败：账户不存在（用统一的错误码，防止用户名枚举攻击）
		l.Warnw("账户不存在",
			"credential_type", string(credDomain.CredPassword),
			"username", passwordCredential.Username,
		)
		return AuthDecision{
			OK:      false,
			ErrCode: ErrInvalidCredential,
		}, nil
	}
	accountID, userID := lookup.AccountID, lookup.UserID

	var principalTenant meta.ID
	switch lookup.AccountType {
	case "opera":
		if lookup.ScopedTenantID.IsZero() {
			l.Warnw("运营账号未配置 scoped_tenant_id",
				"credential_type", string(credDomain.CredPassword),
				"account_id", accountID.String(),
			)
			return AuthDecision{
				OK:      false,
				ErrCode: ErrInvalidCredential,
			}, nil
		}
		if !passwordCredential.TenantID.IsZero() && passwordCredential.TenantID != lookup.ScopedTenantID {
			l.Warnw("登录请求租户与运营账号绑定租户不一致",
				"credential_type", string(credDomain.CredPassword),
				"account_id", accountID.String(),
				"request_tenant_id", passwordCredential.TenantID.String(),
				"scoped_tenant_id", lookup.ScopedTenantID.String(),
			)
			return AuthDecision{
				OK:      false,
				ErrCode: ErrInvalidCredential,
			}, nil
		}
		principalTenant = lookup.ScopedTenantID
	default:
		if !lookup.ScopedTenantID.IsZero() {
			l.Warnw("非运营账号不应设置 scoped_tenant_id",
				"credential_type", string(credDomain.CredPassword),
				"account_id", accountID.String(),
				"type", lookup.AccountType,
			)
			return AuthDecision{
				OK:      false,
				ErrCode: ErrInvalidCredential,
			}, nil
		}
		principalTenant = passwordCredential.TenantID
	}

	l.Debugw("密码认证：步骤2 - 检查账户状态",
		"credential_type", string(credDomain.CredPassword),
		"account_id", accountID.String(),
	)

	// Step 2: 检查账户状态
	enabled, locked, err := p.accountRepo.GetAccountStatus(ctx, accountID)
	if err != nil {
		l.Errorw("查询账户状态失败",
			"credential_type", string(credDomain.CredPassword),
			"account_id", accountID.String(),
			"error", err.Error(),
		)
		return AuthDecision{}, fmt.Errorf("failed to get account status: %w", err)
	}
	if !enabled {
		l.Warnw("账户已禁用",
			"credential_type", string(credDomain.CredPassword),
			"account_id", accountID.String(),
		)
		return AuthDecision{
			OK:      false,
			ErrCode: ErrDisabled,
		}, nil
	}
	if locked {
		l.Warnw("账户已锁定",
			"credential_type", string(credDomain.CredPassword),
			"account_id", accountID.String(),
		)
		return AuthDecision{
			OK:      false,
			ErrCode: ErrLocked,
		}, nil
	}

	l.Debugw("密码认证：步骤3 - 查找密码凭据",
		"credential_type", string(credDomain.CredPassword),
		"account_id", accountID.String(),
	)

	// Step 3: 查找密码凭据
	credentialID, storedHash, err := p.credRepo.FindPasswordCredential(ctx, accountID)
	if err != nil {
		l.Errorw("查询密码凭据失败",
			"credential_type", string(credDomain.CredPassword),
			"account_id", accountID.String(),
			"error", err.Error(),
		)
		return AuthDecision{}, fmt.Errorf("failed to find password credential: %w", err)
	}
	if credentialID.IsZero() {
		// 账户没有设置密码
		l.Warnw("账户未设置密码",
			"credential_type", string(credDomain.CredPassword),
			"account_id", accountID.String(),
		)
		return AuthDecision{
			OK:      false,
			ErrCode: ErrInvalidCredential,
		}, nil
	}

	l.Debugw("密码认证：步骤4 - 验证密码",
		"credential_type", string(credDomain.CredPassword),
		"credential_id", credentialID.String(),
	)

	// Step 4: 验证密码（加上全局pepper）
	plaintextWithPepper := passwordCredential.Password + p.hasher.Pepper()
	if !p.hasher.Verify(storedHash, plaintextWithPepper) {
		// 密码错误（返回凭据ID用于失败次数统计）
		l.Warnw("密码验证失败",
			"credential_type", string(credDomain.CredPassword),
			"credential_id", credentialID.String(),
		)
		return AuthDecision{
			OK:           false,
			ErrCode:      ErrInvalidCredential,
			CredentialID: credentialID,
		}, nil
	}

	l.Debugw("密码认证：步骤5 - 检查是否需要密码rehash",
		"credential_type", string(credDomain.CredPassword),
		"credential_id", credentialID.String(),
	)

	// Step 5: 检查是否需要密码rehash（例如算法参数升级）
	var shouldRotate bool
	var newHashBytes []byte
	if p.hasher.NeedRehash(storedHash) {
		newHash, err := p.hasher.Hash(plaintextWithPepper)
		if err != nil {
			// rehash失败不应该阻止认证成功
			// 记录日志即可，由应用层决定是否处理
			l.Warnw("密码rehash失败",
				"credential_type", string(credDomain.CredPassword),
				"credential_id", credentialID.String(),
				"error", err.Error(),
			)
		} else {
			shouldRotate = true
			newHashBytes = []byte(newHash)
			l.Debugw("检测到需要rehash的密码",
				"credential_type", string(credDomain.CredPassword),
				"credential_id", credentialID.String(),
			)
		}
	}

	// Step 6: 认证成功，构造Principal
	l.Debugw("密码认证成功",
		"credential_type", string(credDomain.CredPassword),
		"account_id", accountID.String(),
		"user_id", userID.String(),
		"should_rotate", shouldRotate,
	)

	principal := &Principal{
		AccountID: accountID,
		UserID:    userID,
		TenantID:  principalTenant,
		AMR:       []string{string(AMRPassword)},
		Claims: map[string]any{
			"auth_time": ctx.Value("request_time"), // 认证时间
		},
	}

	return AuthDecision{
		OK:           true,
		Principal:    principal,
		CredentialID: credentialID,
		ShouldRotate: shouldRotate,
		NewMaterial:  newHashBytes,
	}, nil
}
