package rolebinding

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// Validator 赋权验证器接口。
// 封装赋权相关的验证规则
type Validator interface {
	// ValidateGrantParameters 验证授权参数。
	ValidateGrantParameters(subjectType SubjectType, subjectID meta.ID, roleID meta.ID, tenantID string, grantedBy string) error

	// ValidateRevokeParameters 验证撤销参数。
	ValidateRevokeParameters(subjectType SubjectType, subjectID meta.ID, roleID meta.ID, tenantID string) error

	// CheckRoleExists 检查角色是否存在
	CheckRoleExists(ctx context.Context, roleID meta.ID, tenantID string) error

	// CheckSubjectExists 检查主体是否存在
	CheckSubjectExists(ctx context.Context, subjectType SubjectType, subjectID meta.ID, tenantID string) error
}
