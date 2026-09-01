package authorization

import (
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

type ObjectContext struct {
	ObjectID   string
	Attributes constraint.Attributes
}

func NewObjectContext(objectID string, attributes constraint.Attributes) (ObjectContext, error) {
	objectID = strings.TrimSpace(objectID)
	if len(attributes) > 0 && objectID == "" {
		return ObjectContext{}, perrors.WithCode(code.ErrInvalidArgument, "object id is required when object attributes are supplied")
	}
	copyAttributes := make(constraint.Attributes, len(attributes))
	for key, value := range attributes {
		key = strings.TrimSpace(key)
		if !strings.HasPrefix(key, "object.") || len(key) == len("object.") {
			return ObjectContext{}, perrors.WithCode(code.ErrInvalidArgument, "object attribute key must use object.<name>: %s", key)
		}
		if err := value.Validate(); err != nil {
			return ObjectContext{}, err
		}
		copyAttributes[key] = value
	}
	return ObjectContext{ObjectID: objectID, Attributes: copyAttributes}, nil
}

type Request struct {
	Subject     subject.Ref
	TenantID    tenant.ID
	ResourceKey resource.Pattern
	Action      resource.Action
	Object      ObjectContext
}

func NewRequest(sub subject.Ref, tenantID, resourceKey, action string, object ObjectContext) (Request, error) {
	if sub.IsZero() {
		return Request{}, perrors.WithCode(code.ErrInvalidArgument, "subject is required")
	}
	tenantValue, err := tenant.NewID(tenantID)
	if err != nil {
		return Request{}, err
	}
	resourceKeyValue, err := resource.NewKey(resourceKey)
	if err != nil {
		return Request{}, err
	}
	resourceValue := resource.Pattern(resourceKeyValue)
	actionValue, err := resource.NewAction(action)
	if err != nil {
		return Request{}, err
	}
	object, err = NewObjectContext(object.ObjectID, object.Attributes)
	if err != nil {
		return Request{}, err
	}
	return Request{Subject: sub, TenantID: tenantValue, ResourceKey: resourceValue, Action: actionValue, Object: object}, nil
}

func (r Request) TenantIDString() string { return r.TenantID.String() }

func ValidateAttributes(schema attribute.Schema, attributes constraint.Attributes) error {
	normalized, err := schema.Normalize()
	if err != nil {
		return err
	}
	for key, value := range attributes {
		definition, ok := normalized.Find(key)
		if !ok {
			return perrors.WithCode(code.ErrInvalidArgument, "unsupported object attribute: %s", key)
		}
		if err := value.Validate(); err != nil {
			return err
		}
		if definition.Type != value.Type {
			return perrors.WithCode(code.ErrInvalidArgument, "object attribute type mismatch: %s", key)
		}
		if definition.Type == attribute.TypeString && len(definition.AllowedStringValues) > 0 {
			allowed := false
			for _, candidate := range definition.AllowedStringValues {
				if value.String != nil && candidate == *value.String {
					allowed = true
					break
				}
			}
			if !allowed {
				return perrors.WithCode(code.ErrInvalidArgument, "object attribute value is not allowed: %s", key)
			}
		}
	}
	return nil
}
