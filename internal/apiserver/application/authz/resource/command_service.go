package resource

import (
	"context"

	resourceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
)

// ResourceCatalog manages protected resource definitions.
type ResourceCatalog struct {
	resourceValidator resourceDomain.Validator
	resourceRepo      resourceDomain.Repository
}

func NewResourceCatalog(
	resourceValidator resourceDomain.Validator,
	resourceRepo resourceDomain.Repository,
) *ResourceCatalog {
	return &ResourceCatalog{
		resourceValidator: resourceValidator,
		resourceRepo:      resourceRepo,
	}
}

func (s *ResourceCatalog) CreateResource(
	ctx context.Context,
	cmd CreateResourceCommand,
) (*resourceDomain.Resource, error) {
	if err := s.resourceValidator.ValidateCreateParameters(cmd.Key, cmd.DisplayName, cmd.AppName, cmd.Domain, cmd.Type, cmd.Actions); err != nil {
		return nil, err
	}
	if err := s.resourceValidator.ValidateScopeKinds(cmd.ScopeKinds); err != nil {
		return nil, err
	}

	newResource, err := resourceDomain.NewResource(
		cmd.Key,
		cmd.Actions,
		resourceDomain.WithDisplayName(cmd.DisplayName),
		resourceDomain.WithAppName(cmd.AppName),
		resourceDomain.WithDomain(cmd.Domain),
		resourceDomain.WithType(cmd.Type),
		resourceDomain.WithScopeKinds(cmd.ScopeKinds),
		resourceDomain.WithDescription(cmd.Description),
	)
	if err != nil {
		return nil, err
	}

	if err := s.resourceRepo.Create(ctx, &newResource); err != nil {
		return nil, err
	}

	return &newResource, nil
}

func (s *ResourceCatalog) UpdateResource(
	ctx context.Context,
	cmd UpdateResourceCommand,
) (*resourceDomain.Resource, error) {
	if cmd.Actions != nil {
		if err := s.resourceValidator.ValidateUpdateParameters(cmd.Actions); err != nil {
			return nil, err
		}
	}
	if err := s.resourceValidator.ValidateScopeKinds(cmd.ScopeKinds); err != nil {
		return nil, err
	}

	existingResource, err := s.resourceRepo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	if cmd.DisplayName != nil {
		existingResource.DisplayName = *cmd.DisplayName
	}
	if len(cmd.Actions) > 0 {
		if err := existingResource.ChangeCatalog(cmd.Actions, existingResource.ScopeKinds); err != nil {
			return nil, err
		}
	}
	if len(cmd.ScopeKinds) > 0 {
		if err := existingResource.ChangeCatalog(existingResource.ActionStrings(), cmd.ScopeKinds); err != nil {
			return nil, err
		}
	}
	if cmd.Description != nil {
		existingResource.Description = *cmd.Description
	}

	if err := s.resourceRepo.Update(ctx, existingResource); err != nil {
		return nil, err
	}

	return existingResource, nil
}

func (s *ResourceCatalog) DeleteResource(
	ctx context.Context,
	resourceID resourceDomain.ResourceID,
) error {
	return s.resourceRepo.Delete(ctx, resourceID)
}
