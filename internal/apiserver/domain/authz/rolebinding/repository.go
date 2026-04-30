package rolebinding

import (
	"context"
)

// Repository 赋权仓储接口（Driven Port）
type Repository interface {
	// Create 创建赋权
	Create(ctx context.Context, binding *Binding) error
	// Delete 删除赋权（撤销）
	Delete(ctx context.Context, id BindingID) error
	// DeleteBySubjectAndRole 根据主体和角色删除赋权
	DeleteBySubjectAndRole(ctx context.Context, subjectType SubjectType, subjectID string, roleID uint64, tenantID string) error
	// FindByID 根据ID获取赋权
	FindByID(ctx context.Context, id BindingID) (*Binding, error)
	// ListBySubject 根据主体列出赋权
	ListBySubject(ctx context.Context, subjectType SubjectType, subjectID, tenantID string) ([]*Binding, error)
	// ListByRole 根据角色列出赋权
	ListByRole(ctx context.Context, roleID uint64, tenantID string) ([]*Binding, error)
}
