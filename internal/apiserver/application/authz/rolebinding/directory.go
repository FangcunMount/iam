// Package rolebinding 角色绑定查询应用服务。
package rolebinding

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	bindingDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/rolebinding"
)

// DirectoryService 负责角色绑定读操作。
type DirectoryService struct {
	bindingValidator bindingDomain.Validator
	bindingRepo      bindingDomain.Repository
}

func NewDirectory(
	bindingValidator bindingDomain.Validator,
	bindingRepo bindingDomain.Repository,
) *DirectoryService {
	return &DirectoryService{
		bindingValidator: bindingValidator,
		bindingRepo:      bindingRepo,
	}
}

// ListBySubject 根据主体列出赋权
func (s *DirectoryService) ListBySubject(ctx context.Context, query ListBySubjectQuery) ([]*bindingDomain.Binding, error) {
	// 1. 直接查询赋权列表（验证由领域层Repository处理）
	bindings, err := s.bindingRepo.ListBySubject(ctx, query.SubjectType, query.SubjectID, query.TenantID)
	if err != nil {
		return nil, errors.Wrap(err, "查询赋权列表失败")
	}

	return bindings, nil
}

// ListByRole 根据角色列出赋权
func (s *DirectoryService) ListByRole(ctx context.Context, query ListByRoleQuery) ([]*bindingDomain.Binding, error) {
	// 1. 检查角色是否存在
	if err := s.bindingValidator.CheckRoleExists(ctx, query.RoleID, query.TenantID); err != nil {
		return nil, err
	}

	// 2. 查询赋权列表
	bindings, err := s.bindingRepo.ListByRole(ctx, query.RoleID, query.TenantID)
	if err != nil {
		return nil, errors.Wrap(err, "查询赋权列表失败")
	}

	return bindings, nil
}
