package credential

import (
	"github.com/FangcunMount/component-base/pkg/log"
)

// Lifecycle 凭据生命周期管理
// 职责：管理凭据的启用和禁用
type lifecycle struct{}

// Ensure Lifecycle implements the Lifecycle interface
var _ Lifecycle = (*lifecycle)(nil)

// NewLifecycle creates a new Lifecycle instance
func NewLifecycle() Lifecycle {
	return &lifecycle{}
}

// Enable 启用凭据
func (cl *lifecycle) Enable(c *Credential) {
	if c == nil {
		log.Warn("Cannot enable credential: credential is nil")
		return
	}

	oldStatus := c.Status
	c.Enable()

	log.Infow("Credential enabled",
		"credentialID", c.ID,
		"loginIdentityID", c.LoginIdentityID,
		"oldStatus", oldStatus.String(),
		"newStatus", c.Status.String(),
	)
}

// Disable 禁用凭据
func (cl *lifecycle) Disable(c *Credential) {
	if c == nil {
		log.Warn("Cannot disable credential: credential is nil")
		return
	}

	oldStatus := c.Status
	c.Disable()

	log.Infow("Credential disabled",
		"credentialID", c.ID,
		"loginIdentityID", c.LoginIdentityID,
		"oldStatus", oldStatus.String(),
		"newStatus", c.Status.String(),
	)
}
