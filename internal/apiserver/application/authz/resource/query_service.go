package resource

import (
	"context"

	resourceDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/resource"
)

type ResourceQueryService struct {
	resourceRepo resourceDomain.Repository
}

func NewResourceQueryService(
	resourceRepo resourceDomain.Repository,
) *ResourceQueryService {
	return &ResourceQueryService{
		resourceRepo: resourceRepo,
	}
}

func (s *ResourceQueryService) GetResourceByID(
	ctx context.Context,
	resourceID resourceDomain.ResourceID,
) (*resourceDomain.Resource, error) {
	return s.resourceRepo.FindByID(ctx, resourceID)
}

func (s *ResourceQueryService) GetResourceByKey(
	ctx context.Context,
	key string,
) (*resourceDomain.Resource, error) {
	return s.resourceRepo.FindByKey(ctx, key)
}

func (s *ResourceQueryService) ListResources(
	ctx context.Context,
	query resourceDomain.ListResourcesQuery,
) (*resourceDomain.ListResourcesResult, error) {
	resources, total, err := s.resourceRepo.List(ctx, query)
	if err != nil {
		return nil, err
	}

	return &resourceDomain.ListResourcesResult{
		Resources: resources,
		Total:     total,
	}, nil
}

// ValidateAction 验证动作是否被资源支持
func (s *ResourceQueryService) ValidateAction(
	ctx context.Context,
	resourceKey, action string,
) (bool, error) {
	resource, err := s.resourceRepo.FindByKey(ctx, resourceKey)
	if err != nil {
		return false, err
	}

	// 检查 action 是否在资源的 Actions 列表中
	for _, a := range resource.Actions {
		if a == action {
			return true, nil
		}
	}

	return false, nil
}
