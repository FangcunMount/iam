// Package assignment 角色赋权查询应用服务。
package assignment

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	assignmentDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
)

// DirectoryService 负责角色赋权读操作。
type DirectoryService struct {
	assignmentValidator assignmentDomain.Validator
	assignmentRepo      assignmentDomain.Repository
}

func NewDirectory(
	assignmentValidator assignmentDomain.Validator,
	assignmentRepo assignmentDomain.Repository,
) *DirectoryService {
	return &DirectoryService{
		assignmentValidator: assignmentValidator,
		assignmentRepo:      assignmentRepo,
	}
}

// ListBySubject 根据主体列出赋权
func (s *DirectoryService) ListBySubject(ctx context.Context, query ListBySubjectQuery) ([]*assignmentDomain.Assignment, error) {
	// 1. 直接查询赋权列表（验证由领域层Repository处理）
	assignments, err := s.assignmentRepo.ListBySubject(ctx, query.SubjectType, query.SubjectID, query.TenantID)
	if err != nil {
		return nil, errors.Wrap(err, "查询赋权列表失败")
	}

	return assignments, nil
}

// ListByRole 根据角色列出赋权
func (s *DirectoryService) ListByRole(ctx context.Context, query ListByRoleQuery) ([]*assignmentDomain.Assignment, error) {
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
