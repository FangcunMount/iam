package resource

import (
	"context"

	authzDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz"
	resourceDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/resource"
)

type Catalog interface {
	CreateResource(ctx context.Context, cmd CreateResourceCommand) (*resourceDomain.Resource, error)
	UpdateResource(ctx context.Context, cmd UpdateResourceCommand) (*resourceDomain.Resource, error)
	DeleteResource(ctx context.Context, resourceID resourceDomain.ResourceID) error
}

type Directory interface {
	GetResourceByID(ctx context.Context, resourceID resourceDomain.ResourceID) (*resourceDomain.Resource, error)
	GetResourceByKey(ctx context.Context, key string) (*resourceDomain.Resource, error)
	ListResources(ctx context.Context, query ListResourcesQuery) (*ListResourcesResult, error)
	ValidateAction(ctx context.Context, resourceKey, action string) (bool, error)
}

type CreateResourceCommand struct {
	Key         string
	DisplayName string
	AppName     string
	Domain      string
	Type        string
	Actions     []string
	ScopeKinds  []authzDomain.ScopeKind
	Description string
}

type UpdateResourceCommand struct {
	ID          resourceDomain.ResourceID
	DisplayName *string
	Actions     []string
	ScopeKinds  []authzDomain.ScopeKind
	Description *string
}

type ListResourcesQuery struct {
	AppName string
	Domain  string
	Type    string
	Offset  int
	Limit   int
}

type ListResourcesResult struct {
	Resources []*resourceDomain.Resource
	Total     int64
}
