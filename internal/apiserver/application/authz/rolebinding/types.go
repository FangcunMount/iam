package rolebinding

import (
	"context"
	"strings"

	"github.com/FangcunMount/component-base/pkg/errors"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	bindingDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Commands 承载角色绑定写用例；REST 以 role_id 写入，gRPC 以 role_name 写入。
type Commands interface {
	Grant(ctx context.Context, cmd GrantCommand) (*bindingDomain.Binding, error)
	Revoke(ctx context.Context, cmd RevokeCommand) error
	RevokeByID(ctx context.Context, cmd RevokeByIDCommand) error
	NamedCommands
}

type NamedCommands interface {
	GrantByRoleName(ctx context.Context, cmd GrantByRoleNameCommand) error
	RevokeByRoleName(ctx context.Context, cmd RevokeByRoleNameCommand) error
}

// Directory 承载角色绑定读用例。
type Directory interface {
	ListBySubject(ctx context.Context, query ListBySubjectQuery) ([]*bindingDomain.Binding, error)
	ListByRole(ctx context.Context, query ListByRoleQuery) ([]*bindingDomain.Binding, error)
}

// GrantCommand 授权命令。
type GrantCommand struct {
	SubjectType bindingDomain.SubjectType
	SubjectID   meta.ID
	RoleID      meta.ID
	TenantID    string
	GrantedBy   string
}

func NewGrantCommand(subjectType bindingDomain.SubjectType, subjectID, roleID meta.ID, tenantID, grantedBy string) (GrantCommand, error) {
	tenantIDValue, err := tenant.NewID(tenantID)
	if err != nil {
		return GrantCommand{}, err
	}
	if _, err := bindingDomain.NewBinding(
		subjectType,
		subjectID,
		roleID,
		tenantIDValue.String(),
		bindingDomain.WithGrantedBy(grantedBy),
	); err != nil {
		return GrantCommand{}, err
	}
	return GrantCommand{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		RoleID:      roleID,
		TenantID:    tenantIDValue.String(),
		GrantedBy:   grantedBy,
	}, nil
}

// RevokeCommand 撤销授权命令。
type RevokeCommand struct {
	SubjectType bindingDomain.SubjectType
	SubjectID   meta.ID
	RoleID      meta.ID
	TenantID    string
	ChangedBy   string
	Reason      string
}

func NewRevokeCommand(subjectType bindingDomain.SubjectType, subjectID, roleID meta.ID, tenantID, changedBy, reason string) (RevokeCommand, error) {
	if _, err := subject.NewRef(subject.Type(subjectType), subjectID); err != nil {
		return RevokeCommand{}, err
	}
	if roleID.IsZero() {
		return RevokeCommand{}, errors.WithCode(code.ErrInvalidArgument, "角色ID不能为空")
	}
	tenantIDValue, err := tenant.NewID(tenantID)
	if err != nil {
		return RevokeCommand{}, err
	}
	if strings.TrimSpace(changedBy) == "" {
		return RevokeCommand{}, errors.WithCode(code.ErrInvalidArgument, "changed by is required")
	}
	return RevokeCommand{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		RoleID:      roleID,
		TenantID:    tenantIDValue.String(),
		ChangedBy:   changedBy,
		Reason:      reason,
	}, nil
}

// RevokeByIDCommand 根据 ID 撤销授权命令。
type RevokeByIDCommand struct {
	BindingID bindingDomain.BindingID
	TenantID  string
	ChangedBy string
	Reason    string
}

func NewRevokeByIDCommand(bindingID bindingDomain.BindingID, tenantID, changedBy, reason string) (RevokeByIDCommand, error) {
	if bindingID.Uint64() == 0 {
		return RevokeByIDCommand{}, errors.WithCode(code.ErrInvalidArgument, "赋权ID不能为空")
	}
	tenantIDValue, err := tenant.NewID(tenantID)
	if err != nil {
		return RevokeByIDCommand{}, err
	}
	if strings.TrimSpace(changedBy) == "" {
		return RevokeByIDCommand{}, errors.WithCode(code.ErrInvalidArgument, "changed by is required")
	}
	return RevokeByIDCommand{
		BindingID: bindingID,
		TenantID:  tenantIDValue.String(),
		ChangedBy: changedBy,
		Reason:    reason,
	}, nil
}

func NewGrantByRoleNameCommand(sub subject.Ref, tenantID, roleName, grantedBy string) (GrantByRoleNameCommand, error) {
	if sub.IsZero() {
		return GrantByRoleNameCommand{}, errors.WithCode(code.ErrInvalidArgument, "subject is required")
	}
	tenantIDValue, err := tenant.NewID(tenantID)
	if err != nil {
		return GrantByRoleNameCommand{}, err
	}
	roleNameValue, err := roleDomain.NewName(roleName)
	if err != nil {
		return GrantByRoleNameCommand{}, err
	}
	return GrantByRoleNameCommand{Subject: sub, TenantID: tenantIDValue.String(), RoleName: roleNameValue.String(), GrantedBy: grantedBy}, nil
}

func NewRevokeByRoleNameCommand(sub subject.Ref, tenantID, roleName, changedBy, reason string) (RevokeByRoleNameCommand, error) {
	if sub.IsZero() {
		return RevokeByRoleNameCommand{}, errors.WithCode(code.ErrInvalidArgument, "subject is required")
	}
	tenantIDValue, err := tenant.NewID(tenantID)
	if err != nil {
		return RevokeByRoleNameCommand{}, err
	}
	roleNameValue, err := roleDomain.NewName(roleName)
	if err != nil {
		return RevokeByRoleNameCommand{}, err
	}
	if strings.TrimSpace(changedBy) == "" {
		return RevokeByRoleNameCommand{}, errors.WithCode(code.ErrInvalidArgument, "changed by is required")
	}
	return RevokeByRoleNameCommand{Subject: sub, TenantID: tenantIDValue.String(), RoleName: roleNameValue.String(), ChangedBy: changedBy, Reason: reason}, nil
}

// ListBySubjectQuery 根据主体列出角色绑定查询。
type ListBySubjectQuery struct {
	SubjectType bindingDomain.SubjectType
	SubjectID   meta.ID
	TenantID    string
}

// ListByRoleQuery 根据角色列出角色绑定查询。
type ListByRoleQuery struct {
	RoleID   meta.ID
	TenantID string
}
