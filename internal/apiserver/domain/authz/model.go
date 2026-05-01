// Package authz contains authorization business concepts that are independent
// from the runtime policy engine used to evaluate them.
package authz

import (
	"fmt"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// SubjectType identifies the kind of principal that can receive authorization.
type SubjectType string

const (
	SubjectTypeUser    SubjectType = "user"
	SubjectTypeGroup   SubjectType = "group"
	SubjectTypeService SubjectType = "service"
)

// Subject is the business principal being authorized.
type Subject struct {
	Type SubjectType
	ID   string
}

func NewSubject(subjectType SubjectType, id string) (Subject, error) {
	if strings.TrimSpace(string(subjectType)) == "" {
		return Subject{}, perrors.WithCode(code.ErrInvalidArgument, "subject type is required")
	}
	if strings.TrimSpace(id) == "" {
		return Subject{}, perrors.WithCode(code.ErrInvalidArgument, "subject id is required")
	}
	return Subject{Type: subjectType, ID: strings.TrimSpace(id)}, nil
}

// TenantScope is the tenant/domain boundary for authorization.
type TenantScope struct {
	ID string
}

func NewTenantScope(id string) (TenantScope, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return TenantScope{}, perrors.WithCode(code.ErrInvalidArgument, "tenant id is required")
	}
	return TenantScope{ID: id}, nil
}

// ScopeKind identifies the object range covered by a permission or request.
type ScopeKind string

const (
	ScopeKindAll    ScopeKind = "all"
	ScopeKindOrigin ScopeKind = "origin"
)

const defaultScopeValue = "*"

// Scope describes the object range for an authorization fact.
type Scope struct {
	Kind  ScopeKind
	Value string
}

func DefaultScope() Scope {
	return Scope{Kind: ScopeKindAll, Value: defaultScopeValue}
}

func NewScope(kind ScopeKind, value string) (Scope, error) {
	kind = ScopeKind(strings.TrimSpace(string(kind)))
	value = strings.TrimSpace(value)
	if kind == "" {
		if value != "" {
			return Scope{}, perrors.WithCode(code.ErrInvalidArgument, "scope kind is required when scope value is provided")
		}
		return DefaultScope(), nil
	}
	switch kind {
	case ScopeKindAll:
		if value == "" {
			value = defaultScopeValue
		}
		if value != defaultScopeValue {
			return Scope{}, perrors.WithCode(code.ErrInvalidArgument, "all scope value must be *")
		}
	case ScopeKindOrigin:
		if value == "" || value == defaultScopeValue {
			return Scope{}, perrors.WithCode(code.ErrInvalidArgument, "origin scope value is required")
		}
	default:
		return Scope{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported scope kind: %s", kind)
	}
	return Scope{Kind: kind, Value: value}, nil
}

func NormalizeScope(kind, value string) (Scope, error) {
	return NewScope(ScopeKind(kind), value)
}

func ParseScope(encoded string) (Scope, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return DefaultScope(), nil
	}
	parts := strings.SplitN(encoded, ":", 2)
	if len(parts) != 2 {
		return Scope{}, perrors.WithCode(code.ErrInvalidArgument, "invalid scope format")
	}
	return NewScope(ScopeKind(parts[0]), parts[1])
}

func (s Scope) IsZero() bool {
	return s.Kind == "" && s.Value == ""
}

func (s Scope) Normalized() Scope {
	if s.IsZero() {
		return DefaultScope()
	}
	return s
}

func (s Scope) String() string {
	normalized := s.Normalized()
	return fmt.Sprintf("%s:%s", normalized.Kind, normalized.Value)
}

type PermissionOption func(*Permission)

func WithPermissionScope(scope Scope) PermissionOption {
	return func(p *Permission) {
		p.Scope = scope.Normalized()
	}
}

// Permission states that a role can perform an action on a resource.
type Permission struct {
	RoleName    string
	TenantID    string
	ResourceKey string
	Action      string
	Scope       Scope
}

func NewPermission(roleName, tenantID, resourceKey, action string, opts ...PermissionOption) (Permission, error) {
	roleName = strings.TrimSpace(roleName)
	tenantID = strings.TrimSpace(tenantID)
	resourceKey = strings.TrimSpace(resourceKey)
	action = strings.TrimSpace(action)
	if roleName == "" {
		return Permission{}, perrors.WithCode(code.ErrInvalidArgument, "role name is required")
	}
	if tenantID == "" {
		return Permission{}, perrors.WithCode(code.ErrInvalidArgument, "tenant id is required")
	}
	if resourceKey == "" {
		return Permission{}, perrors.WithCode(code.ErrInvalidArgument, "resource key is required")
	}
	if action == "" {
		return Permission{}, perrors.WithCode(code.ErrInvalidArgument, "action is required")
	}
	permission := Permission{
		RoleName:    roleName,
		TenantID:    tenantID,
		ResourceKey: resourceKey,
		Action:      action,
		Scope:       DefaultScope(),
	}
	for _, opt := range opts {
		opt(&permission)
	}
	if _, err := NewScope(permission.Scope.Kind, permission.Scope.Value); err != nil {
		return Permission{}, err
	}
	permission.Scope = permission.Scope.Normalized()
	return permission, nil
}

// RoleBinding states that a subject holds a role inside a tenant.
type RoleBinding struct {
	Subject   Subject
	RoleName  string
	TenantID  string
	GrantedBy string
}

func NewRoleBinding(subject Subject, roleName, tenantID, grantedBy string) (RoleBinding, error) {
	roleName = strings.TrimSpace(roleName)
	tenantID = strings.TrimSpace(tenantID)
	grantedBy = strings.TrimSpace(grantedBy)
	if subject.Type == "" || subject.ID == "" {
		return RoleBinding{}, perrors.WithCode(code.ErrInvalidArgument, "subject is required")
	}
	if roleName == "" {
		return RoleBinding{}, perrors.WithCode(code.ErrInvalidArgument, "role name is required")
	}
	if tenantID == "" {
		return RoleBinding{}, perrors.WithCode(code.ErrInvalidArgument, "tenant id is required")
	}
	return RoleBinding{Subject: subject, RoleName: roleName, TenantID: tenantID, GrantedBy: grantedBy}, nil
}

type AuthorizationRequestOption func(*AuthorizationRequest)

func WithObjectScope(scope Scope) AuthorizationRequestOption {
	return func(r *AuthorizationRequest) {
		r.ObjectScope = scope.Normalized()
	}
}

// AuthorizationRequest describes the business question "may subject act on resource object range?".
type AuthorizationRequest struct {
	Subject     Subject
	TenantID    string
	ResourceKey string
	Action      string
	ObjectScope Scope
}

func NewAuthorizationRequest(subject Subject, tenantID, resourceKey, action string, opts ...AuthorizationRequestOption) (AuthorizationRequest, error) {
	tenantID = strings.TrimSpace(tenantID)
	resourceKey = strings.TrimSpace(resourceKey)
	action = strings.TrimSpace(action)
	if subject.Type == "" || subject.ID == "" {
		return AuthorizationRequest{}, perrors.WithCode(code.ErrInvalidArgument, "subject is required")
	}
	if tenantID == "" {
		return AuthorizationRequest{}, perrors.WithCode(code.ErrInvalidArgument, "tenant id is required")
	}
	if resourceKey == "" {
		return AuthorizationRequest{}, perrors.WithCode(code.ErrInvalidArgument, "resource key is required")
	}
	if action == "" {
		return AuthorizationRequest{}, perrors.WithCode(code.ErrInvalidArgument, "action is required")
	}
	request := AuthorizationRequest{
		Subject:     subject,
		TenantID:    tenantID,
		ResourceKey: resourceKey,
		Action:      action,
		ObjectScope: DefaultScope(),
	}
	for _, opt := range opts {
		opt(&request)
	}
	if _, err := NewScope(request.ObjectScope.Kind, request.ObjectScope.Value); err != nil {
		return AuthorizationRequest{}, err
	}
	request.ObjectScope = request.ObjectScope.Normalized()
	return request, nil
}

// AuthorizationDecision is the result of an authorization check.
type AuthorizationDecision struct {
	Allowed bool
}
