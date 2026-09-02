package resource

import (
	"context"
	"strings"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	resourceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

type Catalog interface {
	CreateResource(ctx context.Context, cmd CreateResourceCommand) (*resourceDomain.Resource, error)
	UpdateResource(ctx context.Context, cmd UpdateResourceCommand) (*resourceDomain.Resource, error)
	DeleteResource(ctx context.Context, cmd DeleteResourceCommand) error
}

type Directory interface {
	GetResourceByID(ctx context.Context, resourceID resourceDomain.ResourceID) (*resourceDomain.Resource, error)
	GetResourceByKey(ctx context.Context, key string) (*resourceDomain.Resource, error)
	ListResources(ctx context.Context, query ListResourcesQuery) (*ListResourcesResult, error)
	ValidateAction(ctx context.Context, resourceKey, action string) (bool, error)
}

type CreateResourceCommand struct {
	TenantID        string
	ChangedBy       string
	Key             string
	DisplayName     string
	AppName         string
	Domain          string
	Type            string
	Actions         []string
	AttributeSchema attribute.Schema
	Description     string
}

func NewCreateResourceCommand(key, displayName, appName, domain, typ string, actions []string, schema attribute.Schema, description string) (CreateResourceCommand, error) {
	resourceValue, err := resourceDomain.NewResource(
		key,
		actions,
		resourceDomain.WithDisplayName(displayName),
		resourceDomain.WithAppName(appName),
		resourceDomain.WithDomain(domain),
		resourceDomain.WithType(typ),
		resourceDomain.WithAttributeSchema(schema),
		resourceDomain.WithDescription(description),
	)
	if err != nil {
		return CreateResourceCommand{}, err
	}
	return CreateResourceCommand{
		Key:             resourceValue.KeyString(),
		DisplayName:     displayName,
		AppName:         resourceValue.AppName,
		Domain:          resourceValue.Domain,
		Type:            resourceValue.Type,
		Actions:         resourceValue.ActionStrings(),
		AttributeSchema: resourceValue.AttributeSchema,
		Description:     description,
	}, nil
}

type UpdateResourceCommand struct {
	TenantID        string
	ChangedBy       string
	ID              resourceDomain.ResourceID
	DisplayName     *string
	Actions         []string
	AttributeSchema *attribute.Schema
	Description     *string
}

type DeleteResourceCommand struct {
	ID        resourceDomain.ResourceID
	TenantID  string
	ChangedBy string
}

func NewUpdateResourceCommand(id resourceDomain.ResourceID, displayName *string, actions []string, schema *attribute.Schema, description *string) (UpdateResourceCommand, error) {
	if id.Uint64() == 0 {
		return UpdateResourceCommand{}, errors.WithCode(code.ErrInvalidArgument, "资源ID不能为空")
	}
	if displayName != nil && strings.TrimSpace(*displayName) == "" {
		return UpdateResourceCommand{}, errors.WithCode(code.ErrInvalidArgument, "显示名称不能为空")
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
	var normalizedSchema *attribute.Schema
	if schema != nil {
		value, err := schema.Normalize()
		if err != nil {
			return UpdateResourceCommand{}, err
		}
		normalizedSchema = &value
	}
	return UpdateResourceCommand{
		ID:              id,
		DisplayName:     displayName,
		Actions:         actionStrings,
		AttributeSchema: normalizedSchema,
		Description:     description,
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
