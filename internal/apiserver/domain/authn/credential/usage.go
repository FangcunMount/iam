package credential

import (
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// usage 凭据使用记录服务
// 职责：管理凭据的使用状态、成功/失败记录、可用性检查
type usage struct{}

// 确保实现了 Usage 接口
var _ Usage = (*usage)(nil)

// NewUsage 创建凭据使用记录服务实例
func NewUsage() Usage {
	return &usage{}
}

// EnsureUsable 确保凭据可用
// 检查凭据是否处于可用状态（已启用且未锁定）
func (cu *usage) EnsureUsable(c *Credential, now time.Time) error {
	if c == nil {
		return errors.WithCode(code.ErrInvalidCredential, "credential cannot be nil")
	}

	// 使用领域对象的方法检查可用性
	if !c.IsUsable(now) {
		if c.IsDisabled() {
			log.Warnw("Credential is disabled",
				"credentialID", c.ID,
				"status", c.Status.String(),
			)
			return errors.WithCode(code.ErrCredentialDisabled, "credential is disabled")
		}

		if c.IsLockedByTime(now) {
			log.Warnw("Credential is temporarily locked",
				"credentialID", c.ID,
				"lockedUntil", c.LockedUntil,
			)
			return errors.WithCode(code.ErrCredentialLocked, "credential is locked until %s", c.LockedUntil.Format(time.RFC3339))
		}

		return errors.WithCode(code.ErrCredentialNotUsable, "credential is not usable")
	}

	return nil
}

// RecordSuccess 记录认证成功
// 更新最后成功时间，重置失败计数
func (cu *usage) RecordSuccess(c *Credential, now time.Time) {
	if c == nil {
		log.Warn("Cannot record success: credential is nil")
		return
	}

	c.RecordSuccess(now)

	log.Infow("Credential authentication succeeded",
		"credentialID", c.ID,
		"accountID", c.AccountID,
		"lastSuccessAt", now,
	)
}

// RecordFailure 记录认证失败
// 增加失败计数，根据锁定策略决定是否锁定凭据
// 返回是否已锁定
func (cu *usage) RecordFailure(c *Credential, now time.Time, p LockoutPolicy) (locked bool) {
	if c == nil {
		log.Warn("Cannot record failure: credential is nil")
		return false
	}

	failedAttempts := c.RecordFailure(now)

	log.Infow("Credential authentication failed",
		"credentialID", c.ID,
		"accountID", c.AccountID,
		"failedAttempts", failedAttempts,
		"lastFailureAt", now,
	)

	// 应用锁定策略
	if c.ApplyLockPolicy(now, p) {
		log.Warnw("Credential locked due to too many failed attempts",
			"credentialID", c.ID,
			"accountID", c.AccountID,
			"failedAttempts", failedAttempts,
			"threshold", p.Threshold,
			"lockedUntil", c.LockedUntil,
		)
		return true
	}

	return false
}
