package credential

import (
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
)

// locker 凭据锁定器
// 职责：管理凭据的锁定和解锁（行政动作）
type locker struct{}

// Ensure locker implements the Locker interface
var _ Locker = (*locker)(nil)

// NewLocker creates a new locker instance
func NewLocker() Locker {
	return &locker{}
}

// LockUntil 锁定凭据直到指定时间
// 这是行政动作，主要用于 password 类型的凭据
func (cl *locker) LockUntil(c *Credential, until time.Time) {
	if c == nil {
		log.Warn("Cannot lock credential: credential is nil")
		return
	}

	c.LockUntil(until)

	log.Infow("Credential locked administratively",
		"credentialID", c.ID,
		"loginIdentityID", c.LoginIdentityID,
		"lockedUntil", until,
	)
}

// Unlock 解锁凭据
// 清除锁定时间，重置失败计数
func (cl *locker) Unlock(c *Credential) {
	if c == nil {
		log.Warn("Cannot unlock credential: credential is nil")
		return
	}

	c.Unlock()

	log.Infow("Credential unlocked",
		"credentialID", c.ID,
		"loginIdentityID", c.LoginIdentityID,
	)
}
