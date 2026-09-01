package assignment

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Validator 赋权规则验证器（领域服务）。
// 封装赋权相关的业务规则，包括：
// 1. 赋权参数验证
// 2. 角色存在性检查
// 3. 租户隔离检查
// 4. 赋权记录查找
type validator struct {
	assignmentRepo  Repository
	roleRepo        role.Repository
	subjectResolver SubjectResolver
}

// NewValidator 创建赋权规则验证器。
func NewValidator(
	assignmentRepo Repository,
	roleRepo role.Repository,
	userResolver useraccess.UserResolver,
) *validator {
	return NewValidatorWithSubjectResolver(
		assignmentRepo,
		roleRepo,
		NewSubjectResolverRegistry(NewUserSubjectResolver(userResolver)),
	)
}

func NewValidatorWithSubjectResolver(
	assignmentRepo Repository,
	roleRepo role.Repository,
	subjectResolver SubjectResolver,
) *validator {
	return &validator{
		assignmentRepo:  assignmentRepo,
		roleRepo:        roleRepo,
		subjectResolver: subjectResolver,
	}
}

// ValidateGrantParameters 验证授权参数。
func (v *validator) ValidateGrantParameters(
	subjectType SubjectType,
	subjectID meta.ID,
	roleID meta.ID,
	tenantID string,
	grantedBy string,
) error {
	if subjectType == "" {
		return errors.WithCode(code.ErrInvalidArgument, "主体类型不能为空")
	}
	if err := validateWritableSubjectType(subjectType); err != nil {
		return err
	}
	if subjectID.IsZero() {
		return errors.WithCode(code.ErrInvalidArgument, "主体ID不能为空")
	}
	if roleID.IsZero() {
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
	subjectID meta.ID,
	roleID meta.ID,
	tenantID string,
) error {
	if subjectType == "" {
		return errors.WithCode(code.ErrInvalidArgument, "主体类型不能为空")
	}
	if err := validateWritableSubjectType(subjectType); err != nil {
		return err
	}
	if subjectID.IsZero() {
		return errors.WithCode(code.ErrInvalidArgument, "主体ID不能为空")
	}
	if roleID.IsZero() {
		return errors.WithCode(code.ErrInvalidArgument, "角色ID不能为空")
	}
	if tenantID == "" {
		return errors.WithCode(code.ErrInvalidArgument, "租户ID不能为空")
	}
	return nil
}

// CheckRoleExists 检查角色是否存在
func (v *validator) CheckRoleExists(ctx context.Context, roleID meta.ID, tenantID string) error {
	roleExists, err := v.roleRepo.FindByID(ctx, roleID)
	if err != nil {
		if errors.IsCode(err, code.ErrRoleNotFound) {
			return errors.WithCode(code.ErrRoleNotFound, "角色不存在")
		}
		return errors.Wrap(err, "检查角色存在性失败")
	}

	// 验证租户隔离
	if !roleExists.BelongsToTenant(tenantID) {
		return errors.WithCode(code.ErrPermissionDenied, "角色不属于当前租户")
	}

	return nil
}

// CheckSubjectExists 检查主体是否存在
func (v *validator) CheckSubjectExists(ctx context.Context, subjectType SubjectType, subjectID meta.ID, tenantID string) error {
	sub, err := subject.NewRef(subject.Type(subjectType), subjectID)
	if err != nil {
		return err
	}
	tenantIDValue, err := tenant.NewID(tenantID)
	if err != nil {
		return err
	}
	if v.subjectResolver == nil {
		return errors.WithCode(code.ErrInternalServerError, "主体解析器未配置")
	}
	return v.subjectResolver.Resolve(ctx, sub, tenantIDValue)
}

// ValidateRevokeByIDParameters 验证根据ID撤销授权参数
func (v *validator) ValidateRevokeByIDParameters(
	assignmentID AssignmentID,
	tenantID string,
) error {
	if assignmentID.Uint64() == 0 {
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
	roleID meta.ID,
	tenantID string,
) (*role.Role, error) {
	roleExists, err := v.roleRepo.FindByID(ctx, roleID)
	if err != nil {
		if errors.IsCode(err, code.ErrRoleNotFound) {
			return nil, errors.WithCode(code.ErrRoleNotFound, "角色 %d 不存在", roleID.Uint64())
		}
		return nil, errors.Wrap(err, "获取角色失败")
	}

	// 检查租户隔离
	if !roleExists.BelongsToTenant(tenantID) {
		return nil, errors.WithCode(code.ErrPermissionDenied, "无权操作其他租户的角色")
	}

	return roleExists, nil
}

// FindAssignmentBySubjectAndRole 查找主体和角色的赋权记录。
func (v *validator) FindAssignmentBySubjectAndRole(
	ctx context.Context,
	subjectType SubjectType,
	subjectID meta.ID,
	roleID meta.ID,
	tenantID string,
) (*Assignment, error) {
	// 查询赋权列表
	assignments, err := v.assignmentRepo.ListBySubject(ctx, subjectType, subjectID, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "查询赋权记录失败")
	}

	// 查找匹配的赋权记录
	for _, a := range assignments {
		if a.RoleID == roleID {
			return a, nil
		}
	}

	return nil, errors.WithCode(code.ErrAssignmentNotFound, "赋权记录不存在")
}

// GetAssignmentByIDAndCheckTenant 根据 ID 获取赋权记录并检查租户隔离。
func (v *validator) GetAssignmentByIDAndCheckTenant(
	ctx context.Context,
	assignmentID AssignmentID,
	tenantID string,
) (*Assignment, error) {
	// 获取赋权记录
	targetAssignment, err := v.assignmentRepo.FindByID(ctx, assignmentID)
	if err != nil {
		if errors.IsCode(err, code.ErrAssignmentNotFound) {
			return nil, errors.WithCode(code.ErrAssignmentNotFound, "赋权记录不存在")
		}
		return nil, errors.Wrap(err, "获取赋权记录失败")
	}

	// 检查租户隔离
	if !targetAssignment.BelongsToTenant(tenantID) {
		return nil, errors.WithCode(code.ErrPermissionDenied, "无权操作其他租户的赋权记录")
	}

	return targetAssignment, nil
}

// ValidateListBySubjectQuery 验证根据主体查询参数
func (v *validator) ValidateListBySubjectQuery(subjectID meta.ID, tenantID string) error {
	if subjectID.IsZero() {
		return errors.WithCode(code.ErrInvalidArgument, "主体ID不能为空")
	}
	if tenantID == "" {
		return errors.WithCode(code.ErrInvalidArgument, "租户ID不能为空")
	}
	return nil
}

// ValidateListByRoleQuery 验证根据角色查询参数
func (v *validator) ValidateListByRoleQuery(roleID meta.ID, tenantID string) error {
	if roleID.IsZero() {
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
