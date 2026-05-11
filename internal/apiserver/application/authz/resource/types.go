package resource

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	resourceDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
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
	ScopeKinds  []scope.Kind
	Description string
}

func NewCreateResourceCommand(key, displayName, appName, domain, typ string, actions []string, scopeKinds []scope.Kind, description string) (CreateResourceCommand, error) {
	resourceValue, err := resourceDomain.NewResource(
		key,
		actions,
		resourceDomain.WithDisplayName(displayName),
		resourceDomain.WithAppName(appName),
		resourceDomain.WithDomain(domain),
		resourceDomain.WithType(typ),
		resourceDomain.WithScopeKinds(scopeKinds),
		resourceDomain.WithDescription(description),
	)
	if err != nil {
		return CreateResourceCommand{}, err
	}
	normalizedScopeKinds := append([]scope.Kind(nil), resourceValue.ScopeKinds...)
	if len(scopeKinds) == 0 {
		normalizedScopeKinds = nil
	}
	return CreateResourceCommand{
		Key:         resourceValue.KeyString(),
		DisplayName: displayName,
		AppName:     resourceValue.AppName,
		Domain:      resourceValue.Domain,
		Type:        resourceValue.Type,
		Actions:     resourceValue.ActionStrings(),
		ScopeKinds:  normalizedScopeKinds,
		Description: description,
	}, nil
}

type UpdateResourceCommand struct {
	ID          resourceDomain.ResourceID
	DisplayName *string
	Actions     []string
	ScopeKinds  []scope.Kind
	Description *string
}

func NewUpdateResourceCommand(id resourceDomain.ResourceID, displayName *string, actions []string, scopeKinds []scope.Kind, description *string) (UpdateResourceCommand, error) {
	if id.Uint64() == 0 {
		return UpdateResourceCommand{}, errors.WithCode(code.ErrInvalidArgument, "资源ID不能为空")
	}
	var actionStrings []string
	if actions != nil {
		normalizedActions, err := resourceDomain.NormalizeActions(actions)
		if err != nil {
			return UpdateResourceCommand{}, err
		}
		actionStrings = make([]string, 0, len(normalizedActions))
		for _, action := range normalizedActions {
			actionStrings = append(actionStrings, action.String())
		}
	}
	var normalizedScopeKinds []scope.Kind
	if len(scopeKinds) > 0 {
		var err error
		normalizedScopeKinds, err = resourceDomain.NormalizeAndValidateScopeKinds(scopeKinds)
		if err != nil {
			return UpdateResourceCommand{}, err
		}
	}
	return UpdateResourceCommand{
		ID:          id,
		DisplayName: displayName,
		Actions:     actionStrings,
		ScopeKinds:  normalizedScopeKinds,
		Description: description,
	}, nil
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
