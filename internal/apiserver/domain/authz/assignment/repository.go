package assignment

import (
	"context"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Repository 赋权仓储接口（Driven Port）
type Repository interface {
	// Create 创建赋权
	Create(ctx context.Context, assignment *Assignment) error
	// Delete 删除赋权（撤销）
	Delete(ctx context.Context, id AssignmentID) error
	// DeleteBySubjectAndRole 根据主体和角色删除赋权
	DeleteBySubjectAndRole(ctx context.Context, subjectType SubjectType, subjectID meta.ID, roleID meta.ID, tenantID string) error
	// FindByID 根据ID获取赋权
	FindByID(ctx context.Context, id AssignmentID) (*Assignment, error)
	// ListBySubject 根据主体列出赋权
	ListBySubject(ctx context.Context, subjectType SubjectType, subjectID meta.ID, tenantID string) ([]*Assignment, error)
	// ListByRole 根据角色列出赋权
	ListByRole(ctx context.Context, roleID meta.ID, tenantID string) ([]*Assignment, error)
}
