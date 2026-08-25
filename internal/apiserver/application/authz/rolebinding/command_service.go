// Package rolebinding contains the final native role-assignment write use cases.
package rolebinding

import (
	"context"
	"strings"

	"github.com/FangcunMount/component-base/pkg/errors"
	authzshared "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/shared"
	authzuow "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/uow"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	bindingDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
)

type GrantByRoleNameCommand struct {
	Subject                       subject.Ref
	TenantID, RoleName, GrantedBy string
}

type RevokeByRoleNameCommand struct {
	Subject                               subject.Ref
	TenantID, RoleName, ChangedBy, Reason string
}

type CommandService struct {
	validator bindingDomain.Validator
	roles     roleDomain.Repository
	uow       authzuow.UnitOfWork
	reloader  authzshared.RuntimePolicyReloader
}

func NewCommandService(validator bindingDomain.Validator, roles roleDomain.Repository, uow authzuow.UnitOfWork, reloader authzshared.RuntimePolicyReloader) *CommandService {
	return &CommandService{validator: validator, roles: roles, uow: uow, reloader: reloader}
}

func (s *CommandService) Grant(ctx context.Context, cmd GrantCommand) (*bindingDomain.Binding, error) {
	if err := s.validator.ValidateGrantParameters(cmd.SubjectType, cmd.SubjectID, cmd.RoleID, cmd.TenantID, cmd.GrantedBy); err != nil {
		return nil, err
	}
	var created *bindingDomain.Binding
	err := s.commit(ctx, cmd.TenantID, cmd.GrantedBy, "binding grant", func(txCtx context.Context, tx authzuow.TxRepositories) error {
		txValidator := bindingDomain.NewValidator(tx.Bindings, tx.Roles, tx.UserResolver)
		if err := txValidator.CheckRoleExists(txCtx, cmd.RoleID, cmd.TenantID); err != nil {
			return err
		}
		if err := txValidator.CheckSubjectExists(txCtx, cmd.SubjectType, cmd.SubjectID, cmd.TenantID); err != nil {
			return err
		}
		role, err := tx.Roles.FindByIDForUpdate(txCtx, cmd.RoleID)
		if err != nil {
			return errors.Wrap(err, "获取角色失败")
		}
		if !role.BelongsToTenant(cmd.TenantID) {
			return errors.New("角色不属于当前租户")
		}
		binding, err := bindingDomain.NewBinding(cmd.SubjectType, cmd.SubjectID, cmd.RoleID, cmd.TenantID, bindingDomain.WithGrantedBy(cmd.GrantedBy))
		if err != nil {
			return err
		}
		if err := tx.Bindings.Create(txCtx, &binding); err != nil {
			return errors.Wrap(err, "创建赋权失败")
		}
		created = &binding
		return nil
	})
	return created, err
}

func (s *CommandService) Revoke(ctx context.Context, cmd RevokeCommand) error {
	if err := s.validator.ValidateRevokeParameters(cmd.SubjectType, cmd.SubjectID, cmd.RoleID, cmd.TenantID); err != nil {
		return err
	}
	return s.commit(ctx, cmd.TenantID, cmd.ChangedBy, revokeReason(cmd.Reason), func(txCtx context.Context, tx authzuow.TxRepositories) error {
		role, err := tx.Roles.FindByID(txCtx, cmd.RoleID)
		if err != nil {
			return errors.Wrap(err, "获取角色失败")
		}
		if !role.BelongsToTenant(cmd.TenantID) {
			return errors.New("角色不属于当前租户")
		}
		return tx.Bindings.DeleteBySubjectAndRole(txCtx, cmd.SubjectType, cmd.SubjectID, cmd.RoleID, cmd.TenantID)
	})
}

func (s *CommandService) RevokeByID(ctx context.Context, cmd RevokeByIDCommand) error {
	return s.commit(ctx, cmd.TenantID, cmd.ChangedBy, revokeReason(cmd.Reason), func(txCtx context.Context, tx authzuow.TxRepositories) error {
		binding, err := tx.Bindings.FindByID(txCtx, cmd.BindingID)
		if err != nil {
			return errors.Wrap(err, "获取赋权记录失败")
		}
		if !binding.BelongsToTenant(cmd.TenantID) {
			return errors.New("赋权记录不属于当前租户")
		}
		return tx.Bindings.Delete(txCtx, binding.ID)
	})
}

func (s *CommandService) GrantByRoleName(ctx context.Context, cmd GrantByRoleNameCommand) error {
	if s == nil || s.roles == nil {
		return errors.New("role binding command service unavailable")
	}
	role, err := s.roles.FindByName(ctx, cmd.TenantID, cmd.RoleName)
	if err != nil {
		return err
	}
	grant, err := NewGrantCommand(bindingDomain.SubjectType(cmd.Subject.Type), cmd.Subject.ID, role.ID, cmd.TenantID, cmd.GrantedBy)
	if err != nil {
		return err
	}
	_, err = s.Grant(ctx, grant)
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
	revoke, err := NewRevokeCommand(bindingDomain.SubjectType(cmd.Subject.Type), cmd.Subject.ID, role.ID, cmd.TenantID, cmd.ChangedBy, cmd.Reason)
	if err != nil {
		return err
	}
	return s.Revoke(ctx, revoke)
}

func (s *CommandService) commit(ctx context.Context, tenantID, changedBy, reason string, mutation func(context.Context, authzuow.TxRepositories) error) error {
	if s == nil || s.uow == nil {
		return errors.New("role binding command service unavailable")
	}
	if strings.TrimSpace(changedBy) == "" {
		return errors.New("authorization change actor is required")
	}
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) error {
		if err := mutation(txCtx, tx); err != nil {
			return err
		}
		version, err := tx.PolicyVersions.Increment(txCtx, tenantID, changedBy, reason)
		if err != nil {
			return err
		}
		return authzshared.StagePolicyVersionChanged(txCtx, tx.Events, tenantID, version)
	})
	if err != nil {
		return err
	}
	authzshared.ReloadRuntimePolicy(ctx, s.reloader, reason)
	return nil
}

func revokeReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "binding revoke"
	}
	return reason
}
