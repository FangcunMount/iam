// Package rolebinding contains the final native role-assignment write use cases.
package rolebinding

import (
	"context"
	"sort"
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
	_, err := s.commit(ctx, cmd.TenantID, cmd.GrantedBy, "binding grant", func(txCtx context.Context, tx authzuow.TxRepositories) error {
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
	_, err := s.commit(ctx, cmd.TenantID, cmd.ChangedBy, revokeReason(cmd.Reason), func(txCtx context.Context, tx authzuow.TxRepositories) error {
		role, err := tx.Roles.FindByIDForUpdate(txCtx, cmd.RoleID)
		if err != nil {
			return errors.Wrap(err, "获取角色失败")
		}
		if !role.BelongsToTenant(cmd.TenantID) {
			return errors.New("角色不属于当前租户")
		}
		return tx.Bindings.DeleteBySubjectAndRole(txCtx, cmd.SubjectType, cmd.SubjectID, cmd.RoleID, cmd.TenantID)
	})
	return err
}

func (s *CommandService) RevokeByID(ctx context.Context, cmd RevokeByIDCommand) error {
	_, err := s.commit(ctx, cmd.TenantID, cmd.ChangedBy, revokeReason(cmd.Reason), func(txCtx context.Context, tx authzuow.TxRepositories) error {
		binding, err := tx.Bindings.FindByID(txCtx, cmd.BindingID)
		if err != nil {
			return errors.Wrap(err, "获取赋权记录失败")
		}
		if !binding.BelongsToTenant(cmd.TenantID) {
			return errors.New("赋权记录不属于当前租户")
		}
		return tx.Bindings.Delete(txCtx, binding.ID)
	})
	return err
}

func (s *CommandService) GrantByRoleName(ctx context.Context, cmd GrantByRoleNameCommand) (int64, error) {
	if s == nil || s.roles == nil {
		return 0, errors.New("role binding command service unavailable")
	}
	role, err := s.roles.FindByName(ctx, cmd.TenantID, cmd.RoleName)
	if err != nil {
		return 0, err
	}
	grant, err := NewGrantCommand(bindingDomain.SubjectType(cmd.Subject.Type), cmd.Subject.ID, role.ID, cmd.TenantID, cmd.GrantedBy)
	if err != nil {
		return 0, err
	}
	return s.grantWithVersion(ctx, grant)
}

func (s *CommandService) RevokeByRoleName(ctx context.Context, cmd RevokeByRoleNameCommand) (int64, error) {
	if s == nil || s.roles == nil {
		return 0, errors.New("role binding command service unavailable")
	}
	role, err := s.roles.FindByName(ctx, cmd.TenantID, cmd.RoleName)
	if err != nil {
		return 0, err
	}
	revoke, err := NewRevokeCommand(bindingDomain.SubjectType(cmd.Subject.Type), cmd.Subject.ID, role.ID, cmd.TenantID, cmd.ChangedBy, cmd.Reason)
	if err != nil {
		return 0, err
	}
	return s.revokeWithVersion(ctx, revoke)
}

func (s *CommandService) grantWithVersion(ctx context.Context, cmd GrantCommand) (int64, error) {
	if err := s.validator.ValidateGrantParameters(cmd.SubjectType, cmd.SubjectID, cmd.RoleID, cmd.TenantID, cmd.GrantedBy); err != nil {
		return 0, err
	}
	return s.commit(ctx, cmd.TenantID, cmd.GrantedBy, "binding grant", func(txCtx context.Context, tx authzuow.TxRepositories) error {
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
		return tx.Bindings.Create(txCtx, &binding)
	})
}

func (s *CommandService) revokeWithVersion(ctx context.Context, cmd RevokeCommand) (int64, error) {
	if err := s.validator.ValidateRevokeParameters(cmd.SubjectType, cmd.SubjectID, cmd.RoleID, cmd.TenantID); err != nil {
		return 0, err
	}
	return s.commit(ctx, cmd.TenantID, cmd.ChangedBy, revokeReason(cmd.Reason), func(txCtx context.Context, tx authzuow.TxRepositories) error {
		role, err := tx.Roles.FindByIDForUpdate(txCtx, cmd.RoleID)
		if err != nil {
			return errors.Wrap(err, "获取角色失败")
		}
		if !role.BelongsToTenant(cmd.TenantID) {
			return errors.New("角色不属于当前租户")
		}
		return tx.Bindings.DeleteBySubjectAndRole(txCtx, cmd.SubjectType, cmd.SubjectID, cmd.RoleID, cmd.TenantID)
	})
}

func (s *CommandService) ReplaceManagedAssignments(ctx context.Context, cmd ReplaceManagedAssignmentsCommand) (ReplaceManagedAssignmentsResult, error) {
	if s == nil || s.uow == nil {
		return ReplaceManagedAssignmentsResult{}, errors.New("role binding command service unavailable")
	}
	validated, err := NewReplaceManagedAssignmentsCommand(
		cmd.Subject, cmd.TenantID, cmd.RoleNames, cmd.ManagedRoleNames, cmd.ChangedBy, cmd.Reason,
	)
	if err != nil {
		return ReplaceManagedAssignmentsResult{}, err
	}
	cmd = validated
	targetSet := make(map[string]struct{}, len(cmd.RoleNames))
	for _, roleName := range cmd.RoleNames {
		targetSet[roleName] = struct{}{}
	}
	result := ReplaceManagedAssignmentsResult{DirectRoles: append([]string(nil), cmd.RoleNames...)}
	err = s.uow.WithinTx(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) error {
		txValidator := bindingDomain.NewValidator(tx.Bindings, tx.Roles, tx.UserResolver)
		if err := txValidator.CheckSubjectExists(txCtx, bindingDomain.SubjectType(cmd.Subject.Type), cmd.Subject.ID, cmd.TenantID); err != nil {
			return err
		}

		managedRoles := make(map[string]*roleDomain.Role, len(cmd.ManagedRoleNames))
		orderedRoles := make([]*roleDomain.Role, 0, len(cmd.ManagedRoleNames))
		for _, roleName := range cmd.ManagedRoleNames {
			role, err := tx.Roles.FindByName(txCtx, cmd.TenantID, roleName)
			if err != nil {
				return errors.Wrap(err, "find managed role")
			}
			if !role.BelongsToTenant(cmd.TenantID) {
				return errors.New("managed role does not belong to tenant")
			}
			managedRoles[roleName] = role
			orderedRoles = append(orderedRoles, role)
		}
		sort.Slice(orderedRoles, func(i, j int) bool { return orderedRoles[i].ID.Uint64() < orderedRoles[j].ID.Uint64() })
		for _, role := range orderedRoles {
			if _, err := tx.Roles.FindByIDForUpdate(txCtx, role.ID); err != nil {
				return errors.Wrap(err, "lock managed role")
			}
		}

		bindings, err := tx.Bindings.ListBySubject(txCtx, bindingDomain.SubjectType(cmd.Subject.Type), cmd.Subject.ID, cmd.TenantID)
		if err != nil {
			return errors.Wrap(err, "list subject assignments")
		}
		managedByID := make(map[uint64]string, len(managedRoles))
		for roleName, role := range managedRoles {
			managedByID[role.ID.Uint64()] = roleName
		}
		currentManaged := make(map[string]*bindingDomain.Binding)
		for _, binding := range bindings {
			if roleName, ok := managedByID[binding.RoleID.Uint64()]; ok {
				currentManaged[roleName] = binding
			}
		}

		for roleName, binding := range currentManaged {
			if _, keep := targetSet[roleName]; keep {
				continue
			}
			if err := tx.Bindings.Delete(txCtx, binding.ID); err != nil {
				return errors.Wrap(err, "revoke managed assignment")
			}
			result.Changed = true
		}
		for _, roleName := range cmd.RoleNames {
			if _, exists := currentManaged[roleName]; exists {
				continue
			}
			binding, err := bindingDomain.NewBinding(
				bindingDomain.SubjectType(cmd.Subject.Type), cmd.Subject.ID, managedRoles[roleName].ID,
				cmd.TenantID, bindingDomain.WithGrantedBy(cmd.ChangedBy),
			)
			if err != nil {
				return err
			}
			if err := tx.Bindings.Create(txCtx, &binding); err != nil {
				return errors.Wrap(err, "grant managed assignment")
			}
			result.Changed = true
		}

		if !result.Changed {
			version, err := tx.PolicyVersions.GetCurrent(txCtx, cmd.TenantID)
			if err != nil {
				return err
			}
			if version != nil {
				result.PolicyVersion = version.Version
			}
			return nil
		}
		reason := strings.TrimSpace(cmd.Reason)
		if reason == "" {
			reason = "managed assignments replace"
		}
		version, err := tx.PolicyVersions.Increment(txCtx, cmd.TenantID, cmd.ChangedBy, reason)
		if err != nil {
			return err
		}
		if err := authzshared.StagePolicyVersionChanged(txCtx, tx.Events, cmd.TenantID, version); err != nil {
			return err
		}
		result.PolicyVersion = version.Version
		return nil
	})
	if err != nil {
		return ReplaceManagedAssignmentsResult{}, err
	}
	if result.Changed {
		authzshared.ReloadRuntimePolicy(ctx, s.reloader, "managed assignments replace")
	}
	return result, nil
}

func (s *CommandService) commit(ctx context.Context, tenantID, changedBy, reason string, mutation func(context.Context, authzuow.TxRepositories) error) (int64, error) {
	if s == nil || s.uow == nil {
		return 0, errors.New("role binding command service unavailable")
	}
	if strings.TrimSpace(changedBy) == "" {
		return 0, errors.New("authorization change actor is required")
	}
	var committedVersion int64
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) error {
		if err := mutation(txCtx, tx); err != nil {
			return err
		}
		version, err := tx.PolicyVersions.Increment(txCtx, tenantID, changedBy, reason)
		if err != nil {
			return err
		}
		committedVersion = version.Version
		return authzshared.StagePolicyVersionChanged(txCtx, tx.Events, tenantID, version)
	})
	if err != nil {
		return 0, err
	}
	authzshared.ReloadRuntimePolicy(ctx, s.reloader, reason)
	return committedVersion, nil
}

func revokeReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "binding revoke"
	}
	return reason
}
