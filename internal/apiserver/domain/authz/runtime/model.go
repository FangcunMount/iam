package runtime

import (
	"sort"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

type AuthorizationMode string

const (
	ModeUnconditional       AuthorizationMode = "UNCONDITIONAL"
	ModeObjectCheckRequired AuthorizationMode = "OBJECT_CHECK_REQUIRED"
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

type Reason string

const (
	ReasonAllowed          Reason = "allowed"
	ReasonNotMatched       Reason = "not_matched"
	ReasonAttributeMissing Reason = "attribute_missing"
)

const (
	DenyCodePolicyNotMatched = "policy_not_matched"
	DenyCodeAttributeMissing = "attribute_missing"
)

type Decision struct {
	Allowed              bool
	Reason               Reason
	DenyCode             string
	MatchedGrantID       meta.ID
	MatchedRole          string
	PolicyVersion        int64
	MissingAttributeKeys []string
	EvaluatedAt          time.Time
}

func Allow(grantID meta.ID, role string, policyVersion int64, at time.Time) Decision {
	if at.IsZero() {
		at = time.Now()
	}
	return Decision{
		Allowed: true, Reason: ReasonAllowed, MatchedGrantID: grantID,
		MatchedRole: role, PolicyVersion: policyVersion, EvaluatedAt: at,
	}
}

func Deny(policyVersion int64, missing []string, at time.Time) Decision {
	if at.IsZero() {
		at = time.Now()
	}
	missing = uniqueSorted(missing)
	if len(missing) > 0 {
		return Decision{
			Reason: ReasonAttributeMissing, DenyCode: DenyCodeAttributeMissing,
			PolicyVersion: policyVersion, MissingAttributeKeys: missing, EvaluatedAt: at,
		}
	}
	return Decision{
		Reason: ReasonNotMatched, DenyCode: DenyCodePolicyNotMatched,
		PolicyVersion: policyVersion, EvaluatedAt: at,
	}
}

type PermissionEntry struct {
	Resource string
	Action   string
	Mode     AuthorizationMode
}

type SubjectSnapshot struct {
	DirectRoles    []string
	EffectiveRoles []string
	Permissions    []PermissionEntry
	PolicyVersion  int64
}

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

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
