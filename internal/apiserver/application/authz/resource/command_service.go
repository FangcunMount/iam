package resource

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	policychange "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policychange"
	authzuow "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/uow"
	policyDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// ResourceCatalog manages protected resource definitions transactionally with
// policy versions and durable reload notifications.
type ResourceCatalog struct {
	uow                  authzuow.UnitOfWork
	reloader             policychange.RuntimePolicyReloader
	resourceChangePolicy policyDomain.ResourceChangePolicy
}

func NewResourceCatalog(uow authzuow.UnitOfWork, reloader policychange.RuntimePolicyReloader) *ResourceCatalog {
	return &ResourceCatalog{
		uow:                  uow,
		reloader:             reloader,
		resourceChangePolicy: policyDomain.ResourceChangePolicy{},
	}
}

func (s *ResourceCatalog) CreateResource(ctx context.Context, cmd CreateResourceCommand) (*resourceDomain.Resource, error) {
	if err := s.validateChange(cmd.TenantID, cmd.ChangedBy); err != nil {
		return nil, err
	}
	created, err := resourceDomain.NewResource(
		cmd.Key, cmd.Actions,
		resourceDomain.WithDisplayName(cmd.DisplayName), resourceDomain.WithAppName(cmd.AppName),
		resourceDomain.WithDomain(cmd.Domain), resourceDomain.WithType(cmd.Type),
		resourceDomain.WithAttributeSchema(cmd.AttributeSchema), resourceDomain.WithDescription(cmd.Description),
	)
	if err != nil {
		return nil, err
	}
	err = s.uow.WithinTx(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) error {
		if err := tx.Resources.Create(txCtx, &created); err != nil {
			return err
		}
		version, err := tx.PolicyVersions.Increment(txCtx, cmd.TenantID, cmd.ChangedBy, "authorization resource created")
		if err != nil {
			return err
		}
		return policychange.StagePolicyVersionChanged(txCtx, tx.Events, cmd.TenantID, version)
	})
	if err != nil {
		return nil, err
	}
	policychange.ReloadRuntimePolicy(ctx, s.reloader, "authorization_resource_created")
	return &created, nil
}

func (s *ResourceCatalog) UpdateResource(ctx context.Context, cmd UpdateResourceCommand) (*resourceDomain.Resource, error) {
	if err := s.validateChange(cmd.TenantID, cmd.ChangedBy); err != nil {
		return nil, err
	}
	var updated *resourceDomain.Resource
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) error {
		var err error
		updated, err = tx.Resources.FindByIDForUpdate(txCtx, cmd.ID)
		if err != nil {
			return err
		}
		if cmd.DisplayName != nil {
			if err := updated.Rename(*cmd.DisplayName); err != nil {
				return err
			}
		}
		if len(cmd.Actions) > 0 {
			if err := updated.ChangeCatalog(cmd.Actions); err != nil {
				return err
			}
		}
		if cmd.AttributeSchema != nil {
			if err := updated.ChangeAttributeSchema(*cmd.AttributeSchema); err != nil {
				return err
			}
		}
		if cmd.Description != nil {
			updated.ChangeDescription(*cmd.Description)
		}
		grants, err := tx.PermissionGrants.ListActiveByResource(txCtx, cmd.ID)
		if err != nil {
			return err
		}
		if err := s.resourceChangePolicy.ValidateDependencies(*updated, grants); err != nil {
			return err
		}
		if err := tx.Resources.Update(txCtx, updated); err != nil {
			return err
		}
		for _, tenantID := range s.resourceChangePolicy.AffectedResourceTenantIDs(cmd.TenantID, grants) {
			version, err := tx.PolicyVersions.Increment(txCtx, tenantID, cmd.ChangedBy, "authorization resource updated")
			if err != nil {
				return err
			}
			if err := policychange.StagePolicyVersionChanged(txCtx, tx.Events, tenantID, version); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	policychange.ReloadRuntimePolicy(ctx, s.reloader, "authorization_resource_updated")
	return updated, nil
}

func (s *ResourceCatalog) DeleteResource(ctx context.Context, cmd DeleteResourceCommand) error {
	if cmd.ID.Uint64() == 0 {
		return perrors.WithCode(code.ErrInvalidArgument, "resource id is required")
	}
	if err := s.validateChange(cmd.TenantID, cmd.ChangedBy); err != nil {
		return err
	}
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx authzuow.TxRepositories) error {
		if _, err := tx.Resources.FindByIDForUpdate(txCtx, cmd.ID); err != nil {
			return err
		}
		grants, err := tx.PermissionGrants.ListActiveByResource(txCtx, cmd.ID)
		if err != nil {
			return err
		}
		if err := s.resourceChangePolicy.EnsureUnused(grants); err != nil {
			return err
		}
		if err := tx.Resources.Delete(txCtx, cmd.ID); err != nil {
			return err
		}
		version, err := tx.PolicyVersions.Increment(txCtx, cmd.TenantID, cmd.ChangedBy, "authorization resource deleted")
		if err != nil {
			return err
		}
		return policychange.StagePolicyVersionChanged(txCtx, tx.Events, cmd.TenantID, version)
	})
	if err == nil {
		policychange.ReloadRuntimePolicy(ctx, s.reloader, "authorization_resource_deleted")
	}
	return err
}

func (s *ResourceCatalog) validateChange(tenantID, changedBy string) error {
	if s == nil || s.uow == nil {
		return perrors.WithCode(code.ErrInternalServerError, "resource catalog is unavailable")
	}
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(changedBy) == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "tenant and changed by are required")
	}
	return nil
}
