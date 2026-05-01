package policy

import (
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authz "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
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
	TenantID    string
	Actor       Actor
	Reason      string
	Permission  *authz.Permission
	RoleBinding *authz.RoleBinding
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
	scope authz.Scope,
	actor Actor,
	reason string,
) (PolicyChange, error) {
	return permissionChange(PolicyChangeGrantPermission, targetRole, targetResource, action, scope, actor, reason)
}

func (AuthorizationPolicy) RevokePermission(
	targetRole role.Role,
	targetResource resource.Resource,
	action string,
	scope authz.Scope,
	actor Actor,
	reason string,
) (PolicyChange, error) {
	return permissionChange(PolicyChangeRevokePermission, targetRole, targetResource, action, scope, actor, reason)
}

func (AuthorizationPolicy) BindRole(
	subject authz.Subject,
	targetRole role.Role,
	actor Actor,
	reason string,
) (PolicyChange, error) {
	return roleBindingChange(PolicyChangeBindRole, subject, targetRole, actor, reason)
}

func (AuthorizationPolicy) UnbindRole(
	subject authz.Subject,
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
	scope authz.Scope,
	actor Actor,
	reason string,
) (PolicyChange, error) {
	if strings.TrimSpace(targetRole.TenantID) == "" {
		return PolicyChange{}, perrors.WithCode(code.ErrInvalidArgument, "role tenant id is required")
	}
	if !targetResource.HasAction(action) {
		return PolicyChange{}, perrors.WithCode(code.ErrInvalidAction, "Action %s 不被资源 %s 支持", action, targetResource.Key)
	}
	if !targetResource.AllowsScopeKind(scope.Normalized().Kind) {
		return PolicyChange{}, perrors.WithCode(code.ErrInvalidArgument, "资源 %s 不支持 scope %s", targetResource.Key, scope.Normalized().Kind)
	}
	permission, err := authz.NewPermission(
		targetRole.Name,
		targetRole.TenantID,
		targetResource.Key,
		action,
		authz.WithPermissionScope(scope.Normalized()),
	)
	if err != nil {
		return PolicyChange{}, err
	}
	return PolicyChange{
		Kind:       kind,
		TenantID:   targetRole.TenantID,
		Actor:      actor,
		Reason:     reason,
		Permission: &permission,
	}, nil
}

func roleBindingChange(
	kind PolicyChangeKind,
	subject authz.Subject,
	targetRole role.Role,
	actor Actor,
	reason string,
) (PolicyChange, error) {
	binding, err := authz.NewRoleBinding(subject, targetRole.Name, targetRole.TenantID, actor.ID)
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
