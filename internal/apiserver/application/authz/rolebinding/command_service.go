// Package rolebinding 角色绑定命令应用服务。
package rolebinding

import (
	"context"
	"strings"

	"github.com/FangcunMount/component-base/pkg/errors"
	policyApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/policy"
	authzuow "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/uow"
	roleDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	bindingDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/subject"
)

type GrantByRoleNameCommand struct {
	Subject   subject.Ref
	TenantID  string
	RoleName  string
	GrantedBy string
}

type RevokeByRoleNameCommand struct {
	Subject   subject.Ref
	TenantID  string
	RoleName  string
	ChangedBy string
	Reason    string
}

// CommandService 协调领域服务、仓储和授权事实端口，处理角色绑定写操作。
type CommandService struct {
	roles roleDomain.Repository
	admin *policyApp.PolicyAdministration
}

func NewCommandService(
	bindingValidator bindingDomain.Validator,
	roles roleDomain.Repository,
	uow authzuow.UnitOfWork,
	runtimeReloader policyApp.RuntimePolicyReloader,
) *CommandService {
	return &CommandService{
		roles: roles,
		admin: policyApp.NewPolicyAdministration(policyApp.PolicyAdministrationDeps{
			RoleBindingValidator: bindingValidator,
			Roles:                roles,
			UnitOfWork:           uow,
			RuntimeReloader:      runtimeReloader,
		}),
	}
}

// Grant 授权（赋予角色）
func (s *CommandService) Grant(ctx context.Context, cmd GrantCommand) (*bindingDomain.Binding, error) {
	return s.admin.BindRoleToSubject(ctx, cmd.SubjectType, cmd.SubjectID, cmd.RoleID, cmd.TenantID, cmd.GrantedBy)
}

// Revoke 撤销授权（移除角色）
func (s *CommandService) Revoke(ctx context.Context, cmd RevokeCommand) error {
	return s.admin.UnbindRoleFromSubject(ctx, cmd.SubjectType, cmd.SubjectID, cmd.RoleID, cmd.TenantID, cmd.ChangedBy, revokeReason(cmd.Reason))
}

// RevokeByID 根据ID撤销授权
func (s *CommandService) RevokeByID(ctx context.Context, cmd RevokeByIDCommand) error {
	return s.admin.UnbindRoleBindingByID(ctx, cmd.BindingID, cmd.TenantID, cmd.ChangedBy, revokeReason(cmd.Reason))
}

func (s *CommandService) GrantByRoleName(ctx context.Context, cmd GrantByRoleNameCommand) error {
	if s == nil || s.roles == nil {
		return errors.New("role binding command service unavailable")
	}
	role, err := s.roles.FindByName(ctx, cmd.TenantID, cmd.RoleName)
	if err != nil {
		return err
	}
	_, err = s.Grant(ctx, GrantCommand{
		SubjectType: bindingDomain.SubjectType(cmd.Subject.Type),
		SubjectID:   cmd.Subject.ID,
		RoleID:      role.ID,
		TenantID:    cmd.TenantID,
		GrantedBy:   cmd.GrantedBy,
	})
	return err
}

func (s *CommandService) RevokeByRoleName(ctx context.Context, cmd RevokeByRoleNameCommand) error {
	if s == nil || s.roles == nil {
		return errors.New("role binding command service unavailable")
	}
	role, err := s.roles.FindByName(ctx, cmd.TenantID, cmd.RoleName)
	if err != nil {
		return err
	}
	return s.Revoke(ctx, RevokeCommand{
		SubjectType: bindingDomain.SubjectType(cmd.Subject.Type),
		SubjectID:   cmd.Subject.ID,
		RoleID:      role.ID,
		TenantID:    cmd.TenantID,
		ChangedBy:   cmd.ChangedBy,
		Reason:      cmd.Reason,
	})
}

func revokeReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "binding revoke"
	}
	return reason
}
