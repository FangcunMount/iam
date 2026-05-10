package decision

import (
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/permission"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
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
	TenantID    string
	ResourceKey string
	Action      string
	ObjectScope scope.Scope
}

func NewRequest(sub subject.Ref, tenantID, resourceKey, action string, opts ...RequestOption) (Request, error) {
	tenantID = strings.TrimSpace(tenantID)
	resourceKey = strings.TrimSpace(resourceKey)
	action = strings.TrimSpace(action)
	if sub.IsZero() {
		return Request{}, perrors.WithCode(code.ErrInvalidArgument, "subject is required")
	}
	if _, err := tenant.NewID(tenantID); err != nil {
		return Request{}, err
	}
	if _, err := resource.NewPattern(resourceKey); err != nil {
		return Request{}, err
	}
	if action == "" {
		return Request{}, perrors.WithCode(code.ErrInvalidArgument, "action is required")
	}
	request := Request{
		Subject:     sub,
		TenantID:    tenantID,
		ResourceKey: resourceKey,
		Action:      action,
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
		decision.MatchedRole = matched.RoleName
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
