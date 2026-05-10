// Package authz is a compatibility facade for AuthZ domain value objects.
//
// New code should import the semantic child packages directly, for example
// authz/subject, authz/scope, authz/permission, and authz/decision.
package authz

import (
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/decision"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/permission"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type SubjectType = subject.Type

const (
	SubjectTypeUser    = subject.TypeUser
	SubjectTypeGroup   = subject.TypeGroup
	SubjectTypeService = subject.TypeService
)

type Subject = subject.Ref

func NewSubject(subjectType SubjectType, id meta.ID) (Subject, error) {
	return subject.NewRef(subjectType, id)
}

func NewUserSubject(id meta.ID) (Subject, error) {
	return subject.NewUserRef(id)
}

type TenantScope struct {
	ID string
}

func NewTenantScope(id string) (TenantScope, error) {
	tenantID, err := tenant.NewID(id)
	if err != nil {
		return TenantScope{}, err
	}
	return TenantScope{ID: tenantID.String()}, nil
}

type ScopeKind = scope.Kind

const (
	ScopeKindAll    = scope.KindAll
	ScopeKindOrigin = scope.KindOrigin
)

type Scope = scope.Scope

func DefaultScope() Scope {
	return scope.Default()
}

func NewScope(kind ScopeKind, value string) (Scope, error) {
	return scope.New(kind, value)
}

func NormalizeScope(kind, value string) (Scope, error) {
	return scope.Normalize(kind, value)
}

func ParseScope(encoded string) (Scope, error) {
	return scope.Parse(encoded)
}

type PermissionOption = permission.Option

func WithPermissionScope(s Scope) PermissionOption {
	return permission.WithScope(s)
}

type Permission = permission.Permission

func NewPermission(roleName, tenantID, resourceKey, action string, opts ...PermissionOption) (Permission, error) {
	return permission.New(roleName, tenantID, resourceKey, action, opts...)
}

type RoleBinding = rolebinding.Fact

func NewRoleBinding(sub Subject, roleName, tenantID, grantedBy string) (RoleBinding, error) {
	return rolebinding.NewFact(sub, roleName, tenantID, grantedBy)
}

type AuthorizationRequestOption = decision.RequestOption

func WithObjectScope(s Scope) AuthorizationRequestOption {
	return decision.WithObjectScope(s)
}

type AuthorizationRequest = decision.Request

func NewAuthorizationRequest(sub Subject, tenantID, resourceKey, action string, opts ...AuthorizationRequestOption) (AuthorizationRequest, error) {
	return decision.NewRequest(sub, tenantID, resourceKey, action, opts...)
}

type AuthorizationDecision = decision.Decision
