package casbin

import (
	"strings"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/decision"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
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

func RequestFromAuthorizationRequest(request decision.Request) Request {
	return Request{
		Sub:   SubjectKey(request.Subject),
		Dom:   request.TenantIDString(),
		Obj:   request.ResourceKeyString(),
		Act:   request.ActionString(),
		Scope: ScopeKey(request.ObjectScope),
	}
}

func PolicyRuleFromPermission(perm permission.Permission) PolicyRule {
	return PolicyRule{
		Sub:   RoleKey(perm.RoleNameString()),
		Dom:   perm.TenantIDString(),
		Obj:   perm.ResourceKeyString(),
		Act:   perm.ActionString(),
		Scope: ScopeKey(perm.Scope),
	}
}

func GroupingRuleFromRoleBinding(binding rolebinding.Fact) GroupingRule {
	return GroupingRule{
		Sub:  SubjectKey(binding.Subject),
		Role: RoleKey(binding.RoleNameString()),
		Dom:  binding.TenantIDString(),
	}
}

func PermissionFromPolicyRule(rule PolicyRule) (permission.Permission, error) {
	return permission.New(
		RoleNameFromKey(rule.Sub),
		rule.Dom,
		rule.Obj,
		rule.Act,
		permission.WithScope(ScopeFromKey(rule.Scope)),
	)
}

func ScopeKey(scope scope.Scope) string {
	if scope.IsZero() {
		return defaultScopeKey
	}
	return scope.Normalized().String()
}

func ScopeFromKey(value string) scope.Scope {
	scopeValue, err := scope.Parse(value)
	if err != nil {
		return scope.Default()
	}
	return scopeValue.Normalized()
}

func SubjectKey(subject subject.Ref) string {
	return string(subject.Type) + ":" + subject.ID.String()
}

func RoleKey(roleName string) string {
	return "role:" + roleName
}

func RoleNameFromKey(roleKey string) string {
	return strings.TrimPrefix(roleKey, "role:")
}
