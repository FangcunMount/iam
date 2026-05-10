package policy

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	authzuow "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/uow"
	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	policyDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	bindingDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type PolicyAdministrationDeps struct {
	PolicyValidator      policyDomain.Validator
	RoleBindingValidator bindingDomain.Validator
	Roles                roleDomain.Repository
	UnitOfWork           authzuow.UnitOfWork
	RuntimeReloader      RuntimePolicyReloader
}

// PolicyAdministration is the application use case for changing authorization policy.
type PolicyAdministration struct {
	policyValidator      policyDomain.Validator
	roleBindingValidator bindingDomain.Validator
	roles                roleDomain.Repository
	committer            *PolicyChangeCommitter
}

func NewPolicyAdministration(deps PolicyAdministrationDeps) *PolicyAdministration {
	return &PolicyAdministration{
		policyValidator:      deps.PolicyValidator,
		roleBindingValidator: deps.RoleBindingValidator,
		roles:                deps.Roles,
		committer:            NewPolicyChangeCommitter(deps.UnitOfWork, deps.RuntimeReloader),
	}
}

func (s *PolicyAdministration) GrantPermissionToRole(ctx context.Context, cmd AddPermissionCommand) error {
	if err := s.policyValidator.ValidateAddPolicyParameters(cmd.RoleID, cmd.ResourceID, cmd.Action, cmd.TenantID, cmd.ChangedBy); err != nil {
		return err
	}
	scope := cmd.Scope.Normalized()
	if _, err := authzDomain.NewScope(scope.Kind, scope.Value); err != nil {
		return err
	}
	actor, err := policyDomain.NewActor(cmd.ChangedBy)
	if err != nil {
		return err
	}

	return s.committer.Commit(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) (policyDomain.PolicyChange, error) {
		targetRole, targetResource, err := resolvePermissionTargets(txCtx, tx, cmd.RoleID, cmd.ResourceID, cmd.TenantID)
		if err != nil {
			return policyDomain.PolicyChange{}, err
		}
		return policyDomain.NewAuthorizationPolicy().GrantPermission(*targetRole, *targetResource, cmd.Action, scope, actor, cmd.Reason)
	})
}

func (s *PolicyAdministration) RevokePermissionFromRole(ctx context.Context, cmd RemovePermissionCommand) error {
	if err := s.policyValidator.ValidateRemovePolicyParameters(cmd.RoleID, cmd.ResourceID, cmd.Action, cmd.TenantID, cmd.ChangedBy); err != nil {
		return err
	}
	scope := cmd.Scope.Normalized()
	if _, err := authzDomain.NewScope(scope.Kind, scope.Value); err != nil {
		return err
	}
	actor, err := policyDomain.NewActor(cmd.ChangedBy)
	if err != nil {
		return err
	}

	return s.committer.Commit(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) (policyDomain.PolicyChange, error) {
		targetRole, targetResource, err := resolvePermissionTargets(txCtx, tx, cmd.RoleID, cmd.ResourceID, cmd.TenantID)
		if err != nil {
			return policyDomain.PolicyChange{}, err
		}
		return policyDomain.NewAuthorizationPolicy().RevokePermission(*targetRole, *targetResource, cmd.Action, scope, actor, cmd.Reason)
	})
}

func (s *PolicyAdministration) BindRoleToSubject(
	ctx context.Context,
	subjectType bindingDomain.SubjectType,
	subjectID meta.ID,
	roleID meta.ID,
	tenantID string,
	grantedBy string,
) (*bindingDomain.Binding, error) {
	if err := s.roleBindingValidator.ValidateGrantParameters(subjectType, subjectID, roleID, tenantID, grantedBy); err != nil {
		return nil, err
	}
	actor, err := policyDomain.NewActor(grantedBy)
	if err != nil {
		return nil, err
	}

	var newBinding *bindingDomain.Binding
	err = s.committer.Commit(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) (policyDomain.PolicyChange, error) {
		txValidator := bindingDomain.NewValidator(tx.Bindings, tx.Roles, tx.Users)
		if err := txValidator.CheckRoleExists(txCtx, roleID, tenantID); err != nil {
			return policyDomain.PolicyChange{}, err
		}
		if err := txValidator.CheckSubjectExists(txCtx, subjectType, subjectID, tenantID); err != nil {
			return policyDomain.PolicyChange{}, err
		}

		targetRole, err := tx.Roles.FindByID(txCtx, roleID)
		if err != nil {
			return policyDomain.PolicyChange{}, errors.Wrap(err, "获取角色失败")
		}
		if !targetRole.BelongsToTenant(tenantID) {
			return policyDomain.PolicyChange{}, errors.New("角色不属于当前租户")
		}

		subject, err := authzDomain.NewSubject(authzDomain.SubjectType(subjectType), subjectID)
		if err != nil {
			return policyDomain.PolicyChange{}, err
		}
		return policyDomain.NewAuthorizationPolicy().BindRole(subject, *targetRole, actor, "binding grant")
	}, BeforeFacts(func(txCtx context.Context, tx authzuow.TxRepositories, change policyDomain.PolicyChange) error {
		created := bindingDomain.NewBinding(
			subjectType,
			subjectID,
			roleID,
			tenantID,
			bindingDomain.WithGrantedBy(grantedBy),
		)
		if err := tx.Bindings.Create(txCtx, &created); err != nil {
			return errors.Wrap(err, "创建赋权失败")
		}
		newBinding = &created
		return nil
	}))
	if err != nil {
		return nil, err
	}
	return newBinding, nil
}

func (s *PolicyAdministration) UnbindRoleFromSubject(
	ctx context.Context,
	subjectType bindingDomain.SubjectType,
	subjectID meta.ID,
	roleID meta.ID,
	tenantID string,
	changedBy string,
	reason string,
) error {
	if err := s.roleBindingValidator.ValidateRevokeParameters(subjectType, subjectID, roleID, tenantID); err != nil {
		return err
	}
	actor, err := policyDomain.NewActor(changedBy)
	if err != nil {
		return err
	}

	return s.committer.Commit(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) (policyDomain.PolicyChange, error) {
		targetRole, err := tx.Roles.FindByID(txCtx, roleID)
		if err != nil {
			return policyDomain.PolicyChange{}, errors.Wrap(err, "获取角色失败")
		}
		if !targetRole.BelongsToTenant(tenantID) {
			return policyDomain.PolicyChange{}, errors.New("角色不属于当前租户")
		}

		subject, err := authzDomain.NewSubject(authzDomain.SubjectType(subjectType), subjectID)
		if err != nil {
			return policyDomain.PolicyChange{}, err
		}
		return policyDomain.NewAuthorizationPolicy().UnbindRole(subject, *targetRole, actor, reason)
	}, BeforeFacts(func(txCtx context.Context, tx authzuow.TxRepositories, change policyDomain.PolicyChange) error {
		if err := tx.Bindings.DeleteBySubjectAndRole(txCtx, subjectType, subjectID, roleID, tenantID); err != nil {
			return errors.Wrap(err, "删除赋权记录失败")
		}
		return nil
	}))
}

func (s *PolicyAdministration) UnbindRoleBindingByID(
	ctx context.Context,
	bindingID bindingDomain.BindingID,
	tenantID string,
	changedBy string,
	reason string,
) error {
	actor, err := policyDomain.NewActor(changedBy)
	if err != nil {
		return err
	}

	var targetBinding *bindingDomain.Binding
	return s.committer.Commit(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) (policyDomain.PolicyChange, error) {
		var err error
		targetBinding, err = tx.Bindings.FindByID(txCtx, bindingID)
		if err != nil {
			return policyDomain.PolicyChange{}, errors.Wrap(err, "获取赋权记录失败")
		}
		if targetBinding.TenantID != tenantID {
			return policyDomain.PolicyChange{}, errors.New("赋权记录不属于当前租户")
		}

		targetRole, err := tx.Roles.FindByID(txCtx, targetBinding.RoleID)
		if err != nil {
			return policyDomain.PolicyChange{}, errors.Wrap(err, "获取角色失败")
		}
		subject, err := authzDomain.NewSubject(authzDomain.SubjectType(targetBinding.SubjectType), targetBinding.SubjectID)
		if err != nil {
			return policyDomain.PolicyChange{}, err
		}
		return policyDomain.NewAuthorizationPolicy().UnbindRole(subject, *targetRole, actor, reason)
	}, AfterFacts(func(txCtx context.Context, tx authzuow.TxRepositories, change policyDomain.PolicyChange) error {
		if err := tx.Bindings.Delete(txCtx, targetBinding.ID); err != nil {
			return errors.Wrap(err, "删除赋权记录失败")
		}
		return nil
	}))
}

func resolvePermissionTargets(
	ctx context.Context,
	tx authzuow.TxRepositories,
	roleID meta.ID,
	resourceID resourceDomain.ResourceID,
	tenantID string,
) (*roleDomain.Role, *resourceDomain.Resource, error) {
	targetRole, err := tx.Roles.FindByID(ctx, roleID)
	if err != nil {
		return nil, nil, err
	}
	if !targetRole.BelongsToTenant(tenantID) {
		return nil, nil, errors.New("角色不属于当前租户")
	}
	targetResource, err := tx.Resources.FindByID(ctx, resourceID)
	if err != nil {
		return nil, nil, err
	}
	return targetRole, targetResource, nil
}
