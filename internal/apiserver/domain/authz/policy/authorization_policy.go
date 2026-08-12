package policy

import (
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/rolebinding"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// Actor identifies the operator that changes the authorization policy.
type Actor struct {
	ID string
}

func NewActor(id string) (Actor, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Actor{}, perrors.WithCode(code.ErrInvalidArgument, "actor id is required")
	}
	return Actor{ID: id}, nil
}

func (a Actor) String() string {
	return a.ID
}

// PolicyChangeKind describes a business authorization policy mutation.
type PolicyChangeKind string

const (
	PolicyChangeGrantPermission  PolicyChangeKind = "grant_permission"
	PolicyChangeRevokePermission PolicyChangeKind = "revoke_permission"
	PolicyChangeBindRole         PolicyChangeKind = "bind_role"
	PolicyChangeUnbindRole       PolicyChangeKind = "unbind_role"
)

// PolicyChange is the domain result of changing authorization policy.
type PolicyChange struct {
	Kind        PolicyChangeKind
	TenantID    tenant.ID
	Actor       Actor
	Reason      string
	Permission  *permission.Permission
	RoleBinding *rolebinding.Fact
}

func (c PolicyChange) TenantIDString() string {
	return c.TenantID.String()
}

// AuthorizationPolicy owns domain rules for creating authorization facts.
type AuthorizationPolicy struct{}

func NewAuthorizationPolicy() AuthorizationPolicy {
	return AuthorizationPolicy{}
}

func (AuthorizationPolicy) GrantPermission(
	targetRole role.Role,
	targetResource resource.Resource,
	action string,
	scope scope.Scope,
	actor Actor,
	reason string,
) (PolicyChange, error) {
	return permissionChange(PolicyChangeGrantPermission, targetRole, targetResource, action, scope, actor, reason)
}

func (AuthorizationPolicy) RevokePermission(
	targetRole role.Role,
	targetResource resource.Resource,
	action string,
	scope scope.Scope,
	actor Actor,
	reason string,
) (PolicyChange, error) {
	return permissionChange(PolicyChangeRevokePermission, targetRole, targetResource, action, scope, actor, reason)
}

func (AuthorizationPolicy) BindRole(
	subject subject.Ref,
	targetRole role.Role,
	actor Actor,
	reason string,
) (PolicyChange, error) {
	return roleBindingChange(PolicyChangeBindRole, subject, targetRole, actor, reason)
}

func (AuthorizationPolicy) UnbindRole(
	subject subject.Ref,
	targetRole role.Role,
	actor Actor,
	reason string,
) (PolicyChange, error) {
	return roleBindingChange(PolicyChangeUnbindRole, subject, targetRole, actor, reason)
}

func permissionChange(
	kind PolicyChangeKind,
	targetRole role.Role,
	targetResource resource.Resource,
	action string,
	scope scope.Scope,
	actor Actor,
	reason string,
) (PolicyChange, error) {
	if targetRole.TenantID.IsZero() {
		return PolicyChange{}, perrors.WithCode(code.ErrInvalidArgument, "role tenant id is required")
	}
	if !targetResource.HasAction(action) {
		return PolicyChange{}, perrors.WithCode(code.ErrInvalidAction, "Action %s 不被资源 %s 支持", action, targetResource.KeyString())
	}
	if !targetResource.AllowsScopeKind(scope.Normalized().Kind) {
		return PolicyChange{}, perrors.WithCode(code.ErrInvalidArgument, "资源 %s 不支持 scope %s", targetResource.KeyString(), scope.Normalized().Kind)
	}
	permissionValue, err := permission.New(
		targetRole.NameString(),
		targetRole.TenantIDString(),
		targetResource.KeyString(),
		action,
		permission.WithScope(scope.Normalized()),
	)
	if err != nil {
		return PolicyChange{}, err
	}
	return PolicyChange{
		Kind:       kind,
		TenantID:   targetRole.TenantID,
		Actor:      actor,
		Reason:     reason,
		Permission: &permissionValue,
	}, nil
}

func roleBindingChange(
	kind PolicyChangeKind,
	subject subject.Ref,
	targetRole role.Role,
	actor Actor,
	reason string,
) (PolicyChange, error) {
	binding, err := rolebinding.NewFact(subject, targetRole.NameString(), targetRole.TenantIDString(), actor.ID)
	if err != nil {
		return PolicyChange{}, err
	}
	return PolicyChange{
		Kind:        kind,
		TenantID:    targetRole.TenantID,
		Actor:       actor,
		Reason:      reason,
		RoleBinding: &binding,
	}, nil
}
