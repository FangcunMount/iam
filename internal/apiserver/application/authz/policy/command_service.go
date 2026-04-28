package policy

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/log"
	authzshared "github.com/FangcunMount/iam/internal/apiserver/application/authz/shared"
	authzuow "github.com/FangcunMount/iam/internal/apiserver/application/authz/uow"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
)

type PolicyCommandService struct {
	policyValidator policyDomain.Validator
	uow             authzuow.UnitOfWork
	casbinAdapter   policyDomain.CasbinAdapter
	versionNotifier policyDomain.VersionNotifier
}

func NewPolicyCommandService(
	policyValidator policyDomain.Validator,
	uow authzuow.UnitOfWork,
	casbinAdapter policyDomain.CasbinAdapter,
	versionNotifier policyDomain.VersionNotifier,
) *PolicyCommandService {
	return &PolicyCommandService{
		policyValidator: policyValidator,
		uow:             uow,
		casbinAdapter:   casbinAdapter,
		versionNotifier: versionNotifier,
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
	err := s.uow.WithinTx(ctx, func(tx authzuow.TxRepositories) error {
		txValidator := policyDomain.NewValidator(tx.Roles, tx.Resources)
		roleKey, err := txValidator.CheckRoleExistsAndTenant(ctx, cmd.RoleID, cmd.TenantID)
		if err != nil {
			return err
		}
		resourceKey, err := txValidator.CheckResourceExistsAndValidateAction(ctx, cmd.ResourceID, cmd.Action)
		if err != nil {
			return err
		}
		rule := policyDomain.BuildPolicyRule(roleKey, cmd.TenantID, resourceKey, cmd.Action)
		if err := tx.RuleStore.AddPolicy(ctx, rule); err != nil {
			return err
		}
		version, err = tx.PolicyVersions.Increment(ctx, cmd.TenantID, cmd.ChangedBy, cmd.Reason)
		return err
	})
	if err != nil {
		return err
	}

	s.publishVersion(ctx, cmd.TenantID, version)
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
	err := s.uow.WithinTx(ctx, func(tx authzuow.TxRepositories) error {
		txValidator := policyDomain.NewValidator(tx.Roles, tx.Resources)
		roleKey, err := txValidator.CheckRoleExistsAndTenant(ctx, cmd.RoleID, cmd.TenantID)
		if err != nil {
			return err
		}
		resourceKey, err := txValidator.CheckResourceExistsAndValidateAction(ctx, cmd.ResourceID, cmd.Action)
		if err != nil {
			return err
		}
		rule := policyDomain.BuildPolicyRule(roleKey, cmd.TenantID, resourceKey, cmd.Action)
		if err := tx.RuleStore.RemovePolicy(ctx, rule); err != nil {
			return err
		}
		version, err = tx.PolicyVersions.Increment(ctx, cmd.TenantID, cmd.ChangedBy, cmd.Reason)
		return err
	})
	if err != nil {
		return err
	}

	s.publishVersion(ctx, cmd.TenantID, version)
	authzshared.ReloadRuntimePolicy(ctx, s.casbinAdapter, "policy remove")
	return nil
}

func (s *PolicyCommandService) publishVersion(ctx context.Context, tenantID string, version *policyDomain.PolicyVersion) {
	if s.versionNotifier == nil || version == nil {
		return
	}
	if err := s.versionNotifier.Publish(ctx, tenantID, version.Version); err != nil {
		log.Errorw("failed to publish authz policy version", "tenant_id", tenantID, "version", version.Version, "error", err)
	}
}
