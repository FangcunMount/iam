package profile

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ================== Domain Capability Interfaces ==================
// 领域层暴露档案规则能力；外部 DTO 解析、事务、持久化由 application 层编排。

// IDCardUniquenessChecking 检查 Profile 身份证唯一性。
type IDCardUniquenessChecking interface {
	// CheckIDCardUnique 检查身份证是否还没有被其他 Profile 使用
	CheckIDCardUnique(ctx context.Context, idCard meta.IDCard) error
	// CheckIDCardChange 在身份证变更时检查唯一性
	CheckIDCardChange(ctx context.Context, profile *Profile, idCard meta.IDCard) error
}
