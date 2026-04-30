package casbin

import (
	"strings"

	authzDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz"
)

const defaultScopeKey = "all:*"

// Request mirrors the Casbin request model r = sub, dom, obj, act, scope.
type Request struct {
	Sub   string
	Dom   string
	Obj   string
	Act   string
	Scope string
}

// PolicyRule mirrors the Casbin policy model p = sub, dom, obj, act, scope.
type PolicyRule struct {
	Sub   string
	Dom   string
	Obj   string
	Act   string
	Scope string
}

// GroupingRule mirrors the Casbin grouping model g = sub, role, dom.
type GroupingRule struct {
	Sub  string
	Role string
	Dom  string
}

func RequestFromAuthorizationRequest(request authzDomain.AuthorizationRequest) Request {
	return Request{
		Sub:   SubjectKey(request.Subject),
		Dom:   request.TenantID,
		Obj:   request.ResourceKey,
		Act:   request.Action,
		Scope: ScopeKey(request.ObjectScope),
	}
}

func PolicyRuleFromPermission(permission authzDomain.Permission) PolicyRule {
	return PolicyRule{
		Sub:   RoleKey(permission.RoleName),
		Dom:   permission.TenantID,
		Obj:   permission.ResourceKey,
		Act:   permission.Action,
		Scope: ScopeKey(permission.Scope),
	}
}

func GroupingRuleFromRoleBinding(binding authzDomain.RoleBinding) GroupingRule {
	return GroupingRule{
		Sub:  SubjectKey(binding.Subject),
		Role: RoleKey(binding.RoleName),
		Dom:  binding.TenantID,
	}
}

func PermissionFromPolicyRule(rule PolicyRule) authzDomain.Permission {
	permission, _ := authzDomain.NewPermission(
		RoleNameFromKey(rule.Sub),
		rule.Dom,
		rule.Obj,
		rule.Act,
		authzDomain.WithPermissionScope(ScopeFromKey(rule.Scope)),
	)
	return permission
}

func ScopeKey(scope authzDomain.Scope) string {
	if scope.IsZero() {
		return defaultScopeKey
	}
	return scope.Normalized().String()
}

func ScopeFromKey(value string) authzDomain.Scope {
	scope, err := authzDomain.ParseScope(value)
	if err != nil {
		return authzDomain.DefaultScope()
	}
	return scope.Normalized()
}

func SubjectKey(subject authzDomain.Subject) string {
	return string(subject.Type) + ":" + subject.ID
}

func RoleKey(roleName string) string {
	return "role:" + roleName
}

func RoleNameFromKey(roleKey string) string {
	return strings.TrimPrefix(roleKey, "role:")
}
