package role

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	policychange "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policychange"
	authzuow "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/uow"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// RoleCatalog mutates tenant role definitions in the same transaction as the
// policy version and outbox notification.
type RoleCatalog struct {
	roleValidator roleDomain.Validator
	uow           authzuow.UnitOfWork
	reloader      policychange.RuntimePolicyReloader
}

func NewRoleCatalog(roleValidator roleDomain.Validator, uow authzuow.UnitOfWork, reloader policychange.RuntimePolicyReloader) *RoleCatalog {
	return &RoleCatalog{roleValidator: roleValidator, uow: uow, reloader: reloader}
}

func (s *RoleCatalog) CreateRole(ctx context.Context, cmd CreateRoleCommand) (*roleDomain.Role, error) {
	if err := s.validateChange(cmd.TenantIDString(), cmd.ChangedBy); err != nil {
		return nil, err
	}
	if err := s.roleValidator.ValidateCreateParameters(cmd.NameString(), cmd.DisplayName, cmd.TenantIDString()); err != nil {
		return nil, err
	}
	created, err := roleDomain.NewRole(cmd.NameString(), cmd.DisplayName, cmd.TenantIDString(), roleDomain.WithDescription(cmd.Description))
	if err != nil {
		return nil, err
	}
	err = s.uow.WithinTx(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) error {
		if err := tx.Roles.Create(txCtx, &created); err != nil {
			return err
		}
		version, err := tx.PolicyVersions.Increment(txCtx, cmd.TenantIDString(), cmd.ChangedBy, "authorization role created")
		if err != nil {
			return err
		}
		return policychange.StagePolicyVersionChanged(txCtx, tx.Events, cmd.TenantIDString(), version)
	})
	if err != nil {
		return nil, err
	}
	policychange.ReloadRuntimePolicy(ctx, s.reloader, "authorization_role_created")
	return &created, nil
}

func (s *RoleCatalog) UpdateRole(ctx context.Context, cmd UpdateRoleCommand) (*roleDomain.Role, error) {
	if err := s.validateChange(cmd.TenantID, cmd.ChangedBy); err != nil {
		return nil, err
	}
	var updated *roleDomain.Role
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) error {
		var err error
		updated, err = tx.Roles.FindByID(txCtx, cmd.ID)
		if err != nil {
			return err
		}
		if !updated.BelongsToTenant(cmd.TenantID) {
			return perrors.WithCode(code.ErrInvalidArgument, "role does not belong to tenant")
		}
		if cmd.DisplayName != nil {
			if err := updated.Rename(*cmd.DisplayName); err != nil {
				return err
			}
		}
		if cmd.Description != nil {
			updated.Description = *cmd.Description
		}
		if err := tx.Roles.Update(txCtx, updated); err != nil {
			return err
		}
		version, err := tx.PolicyVersions.Increment(txCtx, cmd.TenantID, cmd.ChangedBy, "authorization role updated")
		if err != nil {
			return err
		}
		return policychange.StagePolicyVersionChanged(txCtx, tx.Events, cmd.TenantID, version)
	})
	if err != nil {
		return nil, err
	}
	policychange.ReloadRuntimePolicy(ctx, s.reloader, "authorization_role_updated")
	return updated, nil
}

func (s *RoleCatalog) DeleteRole(ctx context.Context, cmd DeleteRoleCommand) error {
	if cmd.ID.IsZero() {
		return perrors.WithCode(code.ErrInvalidArgument, "role id is required")
	}
	if err := s.validateChange(cmd.TenantID, cmd.ChangedBy); err != nil {
		return err
	}
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) error {
		role, err := tx.Roles.FindByIDForUpdate(txCtx, cmd.ID)
		if err != nil {
			return err
		}
		if !role.BelongsToTenant(cmd.TenantID) {
			return perrors.WithCode(code.ErrInvalidArgument, "role does not belong to tenant")
		}
		assignments, err := tx.Assignments.ListByRole(txCtx, cmd.ID, cmd.TenantID)
		if err != nil {
			return err
		}
		grants, err := tx.PermissionGrants.ListByRole(txCtx, cmd.ID, cmd.TenantID)
		if err != nil {
			return err
		}
		inheritances, err := tx.RoleInheritances.ListActiveByTenant(txCtx, cmd.TenantID)
		if err != nil {
			return err
		}
		if err := policychange.EnsureRoleUnused(cmd.ID, assignments, grants, inheritances); err != nil {
			return err
		}
		if err := tx.Roles.Delete(txCtx, cmd.ID); err != nil {
			return err
		}
		version, err := tx.PolicyVersions.Increment(txCtx, cmd.TenantID, cmd.ChangedBy, "authorization role deleted")
		if err != nil {
			return err
		}
		return policychange.StagePolicyVersionChanged(txCtx, tx.Events, cmd.TenantID, version)
	})
	if err == nil {
		policychange.ReloadRuntimePolicy(ctx, s.reloader, "authorization_role_deleted")
	}
	return err
}

func (s *RoleCatalog) validateChange(tenantID, changedBy string) error {
	if s == nil || s.uow == nil {
		return perrors.WithCode(code.ErrInternalServerError, "role catalog is unavailable")
	}
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(changedBy) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "tenant and changed by are required")
	}
	return nil
}
