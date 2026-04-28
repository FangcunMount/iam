package resource

import (
	"context"

	resourceDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/resource"
)

type ResourceCommandService struct {
	resourceValidator resourceDomain.Validator
	resourceRepo      resourceDomain.Repository
}

func NewResourceCommandService(
	resourceValidator resourceDomain.Validator,
	resourceRepo resourceDomain.Repository,
) *ResourceCommandService {
	return &ResourceCommandService{
		resourceValidator: resourceValidator,
		resourceRepo:      resourceRepo,
	}
}

func (s *ResourceCommandService) CreateResource(
	ctx context.Context,
	cmd resourceDomain.CreateResourceCommand,
) (*resourceDomain.Resource, error) {
	if err := s.resourceValidator.ValidateCreateCommand(cmd); err != nil {
		return nil, err
	}

	newResource := resourceDomain.NewResource(
		cmd.Key,
		cmd.Actions,
		resourceDomain.WithDisplayName(cmd.DisplayName),
		resourceDomain.WithAppName(cmd.AppName),
		resourceDomain.WithDomain(cmd.Domain),
		resourceDomain.WithType(cmd.Type),
		resourceDomain.WithDescription(cmd.Description),
	)

	if err := s.resourceRepo.Create(ctx, &newResource); err != nil {
		return nil, err
	}

	return &newResource, nil
}

func (s *ResourceCommandService) UpdateResource(
	ctx context.Context,
	cmd resourceDomain.UpdateResourceCommand,
) (*resourceDomain.Resource, error) {
	if err := s.resourceValidator.ValidateUpdateCommand(cmd); err != nil {
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
		existingResource.Actions = cmd.Actions
	}
	if cmd.Description != nil {
		existingResource.Description = *cmd.Description
	}

	if err := s.resourceRepo.Update(ctx, existingResource); err != nil {
		return nil, err
	}

	return existingResource, nil
}

func (s *ResourceCommandService) DeleteResource(
	ctx context.Context,
	resourceID resourceDomain.ResourceID,
) error {
	return s.resourceRepo.Delete(ctx, resourceID)
}
