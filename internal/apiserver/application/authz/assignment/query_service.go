// Package assignment 赋权查询应用服务
package assignment

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	assignmentDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/assignment"
)

// AssignmentQueryService 赋权查询服务（实现 AssignmentQueryer 接口）
// 负责赋权的读操作，遵循 CQRS 原则
type AssignmentQueryService struct {
	assignmentValidator assignmentDomain.Validator
	assignmentRepo      assignmentDomain.Repository
}

// NewAssignmentQueryService 创建赋权查询服务
func NewAssignmentQueryService(
	assignmentValidator assignmentDomain.Validator,
	assignmentRepo assignmentDomain.Repository,
) *AssignmentQueryService {
	return &AssignmentQueryService{
		assignmentValidator: assignmentValidator,
		assignmentRepo:      assignmentRepo,
	}
}

// ListBySubject 根据主体列出赋权
func (s *AssignmentQueryService) ListBySubject(ctx context.Context, query assignmentDomain.ListBySubjectQuery) ([]*assignmentDomain.Assignment, error) {
	// 1. 直接查询赋权列表（验证由领域层Repository处理）
	assignments, err := s.assignmentRepo.ListBySubject(ctx, query.SubjectType, query.SubjectID, query.TenantID)
	if err != nil {
		return nil, errors.Wrap(err, "查询赋权列表失败")
	}

	return assignments, nil
}

// ListByRole 根据角色列出赋权
func (s *AssignmentQueryService) ListByRole(ctx context.Context, query assignmentDomain.ListByRoleQuery) ([]*assignmentDomain.Assignment, error) {
	// 1. 检查角色是否存在
	if err := s.assignmentValidator.CheckRoleExists(ctx, query.RoleID, query.TenantID); err != nil {
		return nil, err
	}

	// 2. 查询赋权列表
	assignments, err := s.assignmentRepo.ListByRole(ctx, query.RoleID, query.TenantID)
	if err != nil {
		return nil, errors.Wrap(err, "查询赋权列表失败")
	}

	return assignments, nil
}
