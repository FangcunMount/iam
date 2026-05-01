package rolebinding

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// BindingManager 赋权管理器（领域服务）
// 封装赋权相关的业务规则，包括：
// 1. 赋权参数验证
// 2. 角色存在性检查
// 3. 租户隔离检查
// 4. 赋权记录查找
type validator struct {
	bindingRepo Repository
	roleRepo    role.Repository
	userRepo    userDomain.Repository
}

// NewBindingManager 创建赋权管理器
func NewValidator(
	bindingRepo Repository,
	roleRepo role.Repository,
	userRepo userDomain.Repository,
) *validator {
	return &validator{
		bindingRepo: bindingRepo,
		roleRepo:    roleRepo,
		userRepo:    userRepo,
	}
}

// ValidateGrantParameters 验证授权参数。
func (v *validator) ValidateGrantParameters(
	subjectType SubjectType,
	subjectID string,
	roleID uint64,
	tenantID string,
	grantedBy string,
) error {
	if subjectType == "" {
		return errors.WithCode(code.ErrInvalidArgument, "主体类型不能为空")
	}
	if err := validateWritableSubjectType(subjectType); err != nil {
		return err
	}
	if subjectID == "" {
		return errors.WithCode(code.ErrInvalidArgument, "主体ID不能为空")
	}
	if roleID == 0 {
		return errors.WithCode(code.ErrInvalidArgument, "角色ID不能为空")
	}
	if tenantID == "" {
		return errors.WithCode(code.ErrInvalidArgument, "租户ID不能为空")
	}
	if grantedBy == "" {
		return errors.WithCode(code.ErrInvalidArgument, "授权人不能为空")
	}
	return nil
}

// ValidateRevokeParameters 验证撤销授权参数
func (v *validator) ValidateRevokeParameters(
	subjectType SubjectType,
	subjectID string,
	roleID uint64,
	tenantID string,
) error {
	if subjectType == "" {
		return errors.WithCode(code.ErrInvalidArgument, "主体类型不能为空")
	}
	if err := validateWritableSubjectType(subjectType); err != nil {
		return err
	}
	if subjectID == "" {
		return errors.WithCode(code.ErrInvalidArgument, "主体ID不能为空")
	}
	if roleID == 0 {
		return errors.WithCode(code.ErrInvalidArgument, "角色ID不能为空")
	}
	if tenantID == "" {
		return errors.WithCode(code.ErrInvalidArgument, "租户ID不能为空")
	}
	return nil
}

// CheckRoleExists 检查角色是否存在
func (v *validator) CheckRoleExists(ctx context.Context, roleID uint64, tenantID string) error {
	id := meta.FromUint64(roleID) // roleID 来自请求，必定有效
	roleExists, err := v.roleRepo.FindByID(ctx, id)
	if err != nil {
		if errors.IsCode(err, code.ErrRoleNotFound) {
			return errors.WithCode(code.ErrRoleNotFound, "角色不存在")
		}
		return errors.Wrap(err, "检查角色存在性失败")
	}

	// 验证租户隔离
	if roleExists.TenantID != tenantID {
		return errors.WithCode(code.ErrPermissionDenied, "角色不属于当前租户")
	}

	return nil
}

// CheckSubjectExists 检查主体是否存在
func (v *validator) CheckSubjectExists(ctx context.Context, subjectType SubjectType, subjectID, tenantID string) error {
	if err := validateWritableSubjectType(subjectType); err != nil {
		return err
	}
	if v.userRepo == nil {
		return errors.WithCode(code.ErrInternalServerError, "用户仓储未配置")
	}
	userID, err := meta.ParseID(subjectID)
	if err != nil || userID.IsZero() {
		return errors.WithCode(code.ErrInvalidArgument, "主体ID格式错误")
	}
	userExists, err := v.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.IsCode(err, code.ErrUserNotFound) {
			return errors.WithCode(code.ErrUserNotFound, "用户不存在")
		}
		return errors.Wrap(err, "检查用户存在性失败")
	}
	if userExists == nil {
		return errors.WithCode(code.ErrUserNotFound, "用户不存在")
	}
	return nil
}

// ValidateRevokeByIDParameters 验证根据ID撤销授权参数
func (v *validator) ValidateRevokeByIDParameters(
	bindingID BindingID,
	tenantID string,
) error {
	if bindingID.Uint64() == 0 {
		return errors.WithCode(code.ErrInvalidArgument, "赋权ID不能为空")
	}
	if tenantID == "" {
		return errors.WithCode(code.ErrInvalidArgument, "租户ID不能为空")
	}
	return nil
}

// CheckRoleExistsAndTenant 检查角色是否存在并验证租户隔离
// 返回角色实体用于后续操作
func (v *validator) CheckRoleExistsAndTenant(
	ctx context.Context,
	roleID uint64,
	tenantID string,
) (*role.Role, error) {
	id := meta.FromUint64(roleID) // roleID 来自请求，必定有效
	roleExists, err := v.roleRepo.FindByID(ctx, id)
	if err != nil {
		if errors.IsCode(err, code.ErrRoleNotFound) {
			return nil, errors.WithCode(code.ErrRoleNotFound, "角色 %d 不存在", roleID)
		}
		return nil, errors.Wrap(err, "获取角色失败")
	}

	// 检查租户隔离
	if roleExists.TenantID != tenantID {
		return nil, errors.WithCode(code.ErrPermissionDenied, "无权操作其他租户的角色")
	}

	return roleExists, nil
}

// FindBindingBySubjectAndRole 查找主体和角色的赋权记录
func (v *validator) FindBindingBySubjectAndRole(
	ctx context.Context,
	subjectType SubjectType,
	subjectID string,
	roleID uint64,
	tenantID string,
) (*Binding, error) {
	// 查询赋权列表
	bindings, err := v.bindingRepo.ListBySubject(ctx, subjectType, subjectID, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "查询赋权记录失败")
	}

	// 查找匹配的赋权记录
	for _, a := range bindings {
		if a.RoleID == roleID {
			return a, nil
		}
	}

	return nil, errors.WithCode(code.ErrAssignmentNotFound, "赋权记录不存在")
}

// GetBindingByIDAndCheckTenant 根据ID获取赋权记录并检查租户隔离
func (v *validator) GetBindingByIDAndCheckTenant(
	ctx context.Context,
	bindingID BindingID,
	tenantID string,
) (*Binding, error) {
	// 获取赋权记录
	targetBinding, err := v.bindingRepo.FindByID(ctx, bindingID)
	if err != nil {
		if errors.IsCode(err, code.ErrAssignmentNotFound) {
			return nil, errors.WithCode(code.ErrAssignmentNotFound, "赋权记录不存在")
		}
		return nil, errors.Wrap(err, "获取赋权记录失败")
	}

	// 检查租户隔离
	if targetBinding.TenantID != tenantID {
		return nil, errors.WithCode(code.ErrPermissionDenied, "无权操作其他租户的赋权记录")
	}

	return targetBinding, nil
}

// ValidateListBySubjectQuery 验证根据主体查询参数
func (v *validator) ValidateListBySubjectQuery(subjectID string, tenantID string) error {
	if subjectID == "" {
		return errors.WithCode(code.ErrInvalidArgument, "主体ID不能为空")
	}
	if tenantID == "" {
		return errors.WithCode(code.ErrInvalidArgument, "租户ID不能为空")
	}
	return nil
}

// ValidateListByRoleQuery 验证根据角色查询参数
func (v *validator) ValidateListByRoleQuery(roleID uint64, tenantID string) error {
	if roleID == 0 {
		return errors.WithCode(code.ErrInvalidArgument, "角色ID不能为空")
	}
	if tenantID == "" {
		return errors.WithCode(code.ErrInvalidArgument, "租户ID不能为空")
	}
	return nil
}

func validateWritableSubjectType(subjectType SubjectType) error {
	if subjectType == SubjectTypeUser {
		return nil
	}
	return errors.WithCode(code.ErrInvalidArgument, "主体类型 %s 当前不支持写操作，仅支持 user", subjectType)
}
