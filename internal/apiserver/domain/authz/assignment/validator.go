package assignment

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Validator 赋权规则验证器（领域服务）。
type validator struct {
	roleRepo        role.Repository
	subjectResolver SubjectResolver
}

// NewValidator 创建赋权规则验证器。
func NewValidator(roleRepo role.Repository, subjectResolver SubjectResolver) *validator {
	return NewValidatorWithSubjectResolver(
		roleRepo,
		NewSubjectResolverRegistry(subjectResolver),
	)
}

func NewValidatorWithSubjectResolver(roleRepo role.Repository, subjectResolver SubjectResolver) *validator {
	return &validator{
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

func validateWritableSubjectType(subjectType SubjectType) error {
	if subjectType == SubjectTypeUser {
		return nil
	}
	return errors.WithCode(code.ErrInvalidArgument, "主体类型 %s 当前不支持写操作，仅支持 user", subjectType)
}
