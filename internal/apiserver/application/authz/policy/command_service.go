package policy

import (
	"context"

	authzshared "github.com/FangcunMount/iam/internal/apiserver/application/authz/shared"
	authzuow "github.com/FangcunMount/iam/internal/apiserver/application/authz/uow"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
)

type PolicyCommandService struct {
	policyValidator policyDomain.Validator
	uow             authzuow.UnitOfWork
	casbinAdapter   policyDomain.CasbinAdapter
}

func NewPolicyCommandService(
	policyValidator policyDomain.Validator,
	uow authzuow.UnitOfWork,
	casbinAdapter policyDomain.CasbinAdapter,
) *PolicyCommandService {
	return &PolicyCommandService{
		policyValidator: policyValidator,
		uow:             uow,
		casbinAdapter:   casbinAdapter,
	}
}

func (s *PolicyCommandService) AddPolicyRule(
	ctx context.Context,
	cmd policyDomain.AddPolicyRuleCommand,
) error {
	if err := s.policyValidator.ValidateAddPolicyParameters(cmd.RoleID, cmd.ResourceID, cmd.Action, cmd.TenantID, cmd.ChangedBy); err != nil {
		return err
	}

	var version *policyDomain.PolicyVersion
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) error {
		txValidator := policyDomain.NewValidator(tx.Roles, tx.Resources)
		roleKey, err := txValidator.CheckRoleExistsAndTenant(txCtx, cmd.RoleID, cmd.TenantID)
		if err != nil {
			return err
		}
		resourceKey, err := txValidator.CheckResourceExistsAndValidateAction(txCtx, cmd.ResourceID, cmd.Action)
		if err != nil {
			return err
		}
		rule := policyDomain.BuildPolicyRule(roleKey, cmd.TenantID, resourceKey, cmd.Action)
		if err := tx.RuleStore.AddPolicy(txCtx, rule); err != nil {
			return err
		}
		version, err = tx.PolicyVersions.Increment(txCtx, cmd.TenantID, cmd.ChangedBy, cmd.Reason)
		if err != nil {
			return err
		}
		return authzshared.StagePolicyVersionChanged(txCtx, tx.Events, cmd.TenantID, version)
	})
	if err != nil {
		return err
	}

	authzshared.ReloadRuntimePolicy(ctx, s.casbinAdapter, "policy add")
	return nil
}

func (s *PolicyCommandService) RemovePolicyRule(
	ctx context.Context,
	cmd policyDomain.RemovePolicyRuleCommand,
) error {
	if err := s.policyValidator.ValidateRemovePolicyParameters(cmd.RoleID, cmd.ResourceID, cmd.Action, cmd.TenantID, cmd.ChangedBy); err != nil {
		return err
	}

	var version *policyDomain.PolicyVersion
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) error {
		txValidator := policyDomain.NewValidator(tx.Roles, tx.Resources)
		roleKey, err := txValidator.CheckRoleExistsAndTenant(txCtx, cmd.RoleID, cmd.TenantID)
		if err != nil {
			return err
		}
		resourceKey, err := txValidator.CheckResourceExistsAndValidateAction(txCtx, cmd.ResourceID, cmd.Action)
		if err != nil {
			return err
		}
		rule := policyDomain.BuildPolicyRule(roleKey, cmd.TenantID, resourceKey, cmd.Action)
		if err := tx.RuleStore.RemovePolicy(txCtx, rule); err != nil {
			return err
		}
		version, err = tx.PolicyVersions.Increment(txCtx, cmd.TenantID, cmd.ChangedBy, cmd.Reason)
		if err != nil {
			return err
		}
		return authzshared.StagePolicyVersionChanged(txCtx, tx.Events, cmd.TenantID, version)
	})
	if err != nil {
		return err
	}

	authzshared.ReloadRuntimePolicy(ctx, s.casbinAdapter, "policy remove")
	return nil
}
