package credential

import (
	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// rotator 凭据轮换器
// 职责：管理凭据材料的轮换（主要用于 password 类型的条件再哈希）
type rotator struct{}

// Ensure rotator implements the Rotator interface
var _ Rotator = (*rotator)(nil)

// NewRotator creates a new rotator instance
func NewRotator() Rotator {
	return &rotator{}
}

// Rotate 轮换凭据材料
// 主要用于 password 类型的密码更新或条件再哈希
// 参数：
//   - c: 凭据实体
//   - newMaterial: 新的密钥材料（如新的密码哈希）
//   - newAlgo: 新的算法（可选，如果为 nil 则保持原算法）
func (cr *rotator) Rotate(c *Credential, newMaterial []byte, newAlgo *string) {
	if c == nil {
		log.Warn("Cannot rotate credential: credential is nil")
		return
	}

	if len(newMaterial) == 0 {
		log.Warnw("Cannot rotate credential: new material is empty",
			"credentialID", c.ID,
		)
		return
	}

	oldAlgo := ""
	if c.Algo != nil {
		oldAlgo = *c.Algo
	}

	// 委托给领域对象进行轮换
	c.RotateMaterial(newMaterial, newAlgo)

	newAlgoStr := ""
	if c.Algo != nil {
		newAlgoStr = *c.Algo
	}

	log.Infow("Credential material rotated",
		"credentialID", c.ID,
		"loginIdentityID", c.LoginIdentityID,
		"oldAlgo", oldAlgo,
		"newAlgo", newAlgoStr,
	)
}

// ValidateRotation 验证轮换操作的合法性
// 仅 password 类型的凭据可以进行材料轮换
func (cr *rotator) ValidateRotation(c *Credential, newMaterial []byte, newAlgo *string) error {
	if c == nil {
		return errors.WithCode(code.ErrInvalidCredential, "credential cannot be nil")
	}

	// 只有已启用的凭据可以轮换
	if !c.IsEnabled() {
		return errors.WithCode(code.ErrCredentialDisabled, "only enabled credentials can be rotated")
	}

	// 验证新材料不为空
	if len(newMaterial) == 0 {
		return errors.WithCode(code.ErrInvalidCredential, "new material cannot be empty")
	}

	// 如果提供了新算法，验证其不为空
	if newAlgo != nil && *newAlgo == "" {
		return errors.WithCode(code.ErrInvalidCredential, "new algo cannot be empty string")
	}

	return nil
}
