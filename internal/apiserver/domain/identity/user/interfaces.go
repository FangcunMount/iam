package user

import (
	"context"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// ================== Domain Capability Interfaces ==================
// 领域层暴露的是领域能力契约；加载实体、开启事务、持久化等编排留在 application 层。

// PhoneUniquenessChecking 检查 User 手机号唯一性。
type PhoneUniquenessChecking interface {
	// CheckPhoneUnique 检查手机号唯一性
	CheckPhoneUnique(ctx context.Context, phone meta.Phone) error
	// CheckPhoneChange 在手机号变更时检查唯一性
	CheckPhoneChange(ctx context.Context, user *User, phone meta.Phone) error
}
