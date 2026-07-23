package credential

import (
	"time"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// Credential 凭据实体
type Credential struct {
	ID              meta.ID        // 凭据ID
	LoginIdentityID meta.ID        // 归属的登录身份ID
	Type            CredentialType // 凭据类型

	// —— 长期认证材料 —— //
	Material   []byte  // password hash；未来可承载 passkey public key / encrypted secret
	Algo       *string // 算法，例如 argon2id / bcrypt / es256 等
	ParamsJSON []byte  // 低频参数或元数据，例如 hash params / authenticator metadata

	// —— 认证材料状态 —— //
	Status         CredentialStatus // 凭据状态
	FailedAttempts int              // 失败尝试次数
	LockedUntil    *time.Time       // 锁定截止时间
	LastSuccessAt  *time.Time       // 最近成功时间
	LastFailureAt  *time.Time       // 最近失败时间
}

// ==================== 状态查询方法 ====================

// IsEnabled 是否已启用
func (c *Credential) IsEnabled() bool {
	return c.Status == CredStatusEnabled
}

// IsDisabled 是否已禁用
func (c *Credential) IsDisabled() bool {
	return c.Status == CredStatusDisabled
}

// IsLockedByTime 是否被时间锁定
func (c *Credential) IsLockedByTime(now time.Time) bool {
	return c.LockedUntil != nil && now.Before(*c.LockedUntil)
}

// IsUsable 是否可用（已启用且未被锁定）
func (c *Credential) IsUsable(now time.Time) bool {
	return c.IsEnabled() && !c.IsLockedByTime(now)
}

// IsPasswordType 是否为密码类型凭据
func (c *Credential) IsPasswordType() bool {
	return c.Type == CredPassword
}

// ==================== 行为方法 ====================

// RecordSuccess 记录认证成功
func (c *Credential) RecordSuccess(now time.Time) {
	c.LastSuccessAt = &now
	c.FailedAttempts = 0 // 重置失败计数
}

// RecordFailure 记录认证失败，返回当前失败次数
func (c *Credential) RecordFailure(now time.Time) int {
	c.LastFailureAt = &now
	c.FailedAttempts++
	return c.FailedAttempts
}

// LockUntil 锁定凭据直到指定时间
func (c *Credential) LockUntil(until time.Time) {
	c.LockedUntil = &until
}

// Unlock 解锁凭据
func (c *Credential) Unlock() {
	c.LockedUntil = nil
	c.FailedAttempts = 0
}

// ShouldLock 判断是否应该锁定（基于失败次数和策略）
func (c *Credential) ShouldLock(threshold int) bool {
	return c.FailedAttempts >= threshold
}

// ApplyLockPolicy 应用锁定策略，如果达到阈值则自动锁定
// 返回是否已锁定
func (c *Credential) ApplyLockPolicy(now time.Time, policy LockoutPolicy) bool {
	if !policy.Enabled {
		return false
	}

	if !c.IsLockedByTime(now) && c.ShouldLock(policy.Threshold) {
		until := now.Add(policy.LockDuration)
		c.LockUntil(until)
		return true
	}

	return false
}

// Enable 启用凭据
func (c *Credential) Enable() {
	c.Status = CredStatusEnabled
	// 启用时清除锁定
	c.Unlock()
}

// Disable 禁用凭据
func (c *Credential) Disable() {
	c.Status = CredStatusDisabled
}

// RotateMaterial 轮换凭据材料
func (c *Credential) RotateMaterial(newMaterial []byte, newAlgo *string) {
	c.Material = newMaterial
	if newAlgo != nil {
		c.Algo = newAlgo
	}
}

// ==================== 工厂方法 ====================

// NewPasswordCredential 创建密码类型凭据。
func NewPasswordCredential(loginIdentityID meta.ID, material []byte, algo string) *Credential {
	return &Credential{
		LoginIdentityID: loginIdentityID,
		Type:            CredPassword,
		Material:        material,
		Algo:            &algo,
		Status:          CredStatusEnabled,
		FailedAttempts:  0,
	}
}
