package credential

import (
	"time"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// Credential 凭据实体
type Credential struct {
	ID        meta.ID
	AccountID meta.ID

	// —— 外部身份三元组：仅 OAuth/Phone 有值；password 留空 —— //
	IDP           *string // "wechat"|"wecom"|"phone" | nil(本地)
	IDPIdentifier string  // unionid | openid@appid | open_userid | +E164 | ""(password)
	AppID         *string // wechat=appid | wecom=corp_id | nil(本地)

	// —— 三件套（仅 password 会使用；其余类型为空） —— //
	Material   []byte  // PHC 哈希（password）；其余类型 NULL
	Algo       *string // "argon2id"/"bcrypt"；其余类型 NULL
	ParamsJSON []byte  // 低频元数据（如 wx.profile / wecom.agentid / phone 场景）

	// —— 通用状态；只有 password 实际用到失败计数/锁定 —— //
	Status         CredentialStatus
	FailedAttempts int        // 失败尝试次数
	LockedUntil    *time.Time // 锁定截止时间
	LastSuccessAt  *time.Time // 最近成功时间
	LastFailureAt  *time.Time // 最近失败时间
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

// IsLockedByTime 是否被时间锁定（主要用于 password）
func (c *Credential) IsLockedByTime(now time.Time) bool {
	return c.LockedUntil != nil && now.Before(*c.LockedUntil)
}

// IsUsable 是否可用（已启用且未被锁定）
func (c *Credential) IsUsable(now time.Time) bool {
	return c.IsEnabled() && !c.IsLockedByTime(now)
}

// ==================== 类型判断方法 ====================

// IsPasswordType 是否为密码类型凭据
func (c *Credential) IsPasswordType() bool {
	return c.IDP == nil && len(c.Material) > 0 && c.Algo != nil
}

// IsOAuthType 是否为 OAuth 类型凭据
func (c *Credential) IsOAuthType() bool {
	return c.IDP != nil && c.AppID != nil && c.IDPIdentifier != ""
}

// IsPhoneOTPType 是否为手机号 OTP 类型凭据
func (c *Credential) IsPhoneOTPType() bool {
	return c.IDP != nil && *c.IDP == "phone" && c.IDPIdentifier != ""
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

	if c.ShouldLock(policy.Threshold) {
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

// RotateMaterial 轮换凭据材料（主要用于 password）
func (c *Credential) RotateMaterial(newMaterial []byte, newAlgo *string) {
	c.Material = newMaterial
	if newAlgo != nil {
		c.Algo = newAlgo
	}
}

// UpdateIDPIdentifier 更新 IDP 标识符（用于 OAuth 场景的 unionid 更新等）
func (c *Credential) UpdateIDPIdentifier(identifier string) {
	c.IDPIdentifier = identifier
}

// UpdateParams 更新参数（用于更新扩展信息）
func (c *Credential) UpdateParams(params []byte) {
	c.ParamsJSON = params
}

// ==================== 工厂方法 ====================

// NewPasswordCredential 创建密码类型凭据
func NewPasswordCredential(accountID meta.ID, material []byte, algo string) *Credential {
	return &Credential{
		AccountID:      accountID,
		Material:       material,
		Algo:           &algo,
		Status:         CredStatusEnabled,
		FailedAttempts: 0,
	}
}

// NewOAuthCredential 创建 OAuth 类型凭据
func NewOAuthCredential(accountID meta.ID, idp, identifier, appID string, params []byte) *Credential {
	return &Credential{
		AccountID:     accountID,
		IDP:           &idp,
		IDPIdentifier: identifier,
		AppID:         &appID,
		ParamsJSON:    params,
		Status:        CredStatusEnabled,
	}
}

// NewPhoneOTPCredential 创建手机号 OTP 凭据
func NewPhoneOTPCredential(accountID meta.ID, phoneNumber string) *Credential {
	idp := "phone"
	return &Credential{
		AccountID:     accountID,
		IDP:           &idp,
		IDPIdentifier: phoneNumber,
		Status:        CredStatusEnabled,
	}
}
