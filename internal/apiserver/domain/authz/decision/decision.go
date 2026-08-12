package decision

import (
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

type RequestOption func(*Request)

func WithObjectScope(s scope.Scope) RequestOption {
	return func(r *Request) {
		r.ObjectScope = s.Normalized()
	}
}

// Request describes the business question "may subject act on resource object range?".
type Request struct {
	Subject     subject.Ref
	TenantID    tenant.ID
	ResourceKey resource.Pattern
	Action      resource.Action
	ObjectScope scope.Scope
}

func NewRequest(sub subject.Ref, tenantID, resourceKey, action string, opts ...RequestOption) (Request, error) {
	if sub.IsZero() {
		return Request{}, perrors.WithCode(code.ErrInvalidArgument, "subject is required")
	}
	tenantIDValue, err := tenant.NewID(tenantID)
	if err != nil {
		return Request{}, err
	}
	resourcePattern, err := resource.NewPattern(resourceKey)
	if err != nil {
		return Request{}, err
	}
	actionValue, err := resource.NewAction(action)
	if err != nil {
		return Request{}, err
	}
	request := Request{
		Subject:     sub,
		TenantID:    tenantIDValue,
		ResourceKey: resourcePattern,
		Action:      actionValue,
		ObjectScope: scope.Default(),
	}
	for _, opt := range opts {
		opt(&request)
	}
	if _, err := scope.New(request.ObjectScope.Kind, request.ObjectScope.Value); err != nil {
		return Request{}, err
	}
	request.ObjectScope = request.ObjectScope.Normalized()
	return request, nil
}

func (r Request) TenantIDString() string {
	return r.TenantID.String()
}

func (r Request) ResourceKeyString() string {
	return r.ResourceKey.String()
}

func (r Request) ActionString() string {
	return r.Action.String()
}

type Reason string

const (
	ReasonAllowed    Reason = "allowed"
	ReasonNotMatched Reason = "not_matched"
)

const DenyCodePolicyNotMatched = "policy_not_matched"

// Decision is the result of an authorization check.
type Decision struct {
	Allowed           bool
	Reason            Reason
	DenyCode          string
	MatchedRole       string
	MatchedPermission *permission.Permission
	PolicyVersion     int64
	EvaluatedAt       time.Time
}

func Allow(matched *permission.Permission, evaluatedAt time.Time) Decision {
	if evaluatedAt.IsZero() {
		evaluatedAt = time.Now()
	}
	decision := Decision{Allowed: true, Reason: ReasonAllowed, EvaluatedAt: evaluatedAt}
	if matched != nil {
		decision.MatchedPermission = matched
		decision.MatchedRole = matched.RoleNameString()
	}
	return decision
}

func Deny(evaluatedAt time.Time) Decision {
	if evaluatedAt.IsZero() {
		evaluatedAt = time.Now()
	}
	return Decision{
		Allowed:     false,
		Reason:      ReasonNotMatched,
		DenyCode:    DenyCodePolicyNotMatched,
		EvaluatedAt: evaluatedAt,
	}
}
