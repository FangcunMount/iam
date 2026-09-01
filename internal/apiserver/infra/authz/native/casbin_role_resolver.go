package native

import (
	"sort"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	defaultrolemanager "github.com/casbin/casbin/v2/rbac/default-role-manager"
)

type casbinRoleResolver struct {
	manager *defaultrolemanager.RoleManager
}

var _ authorization.RoleResolver = (*casbinRoleResolver)(nil)

func newCasbinRoleResolver(maxHierarchyLevel int) *casbinRoleResolver {
	return &casbinRoleResolver{manager: defaultrolemanager.NewRoleManager(maxHierarchyLevel)}
}

func (r *casbinRoleResolver) addAssignment(sub subject.Ref, roleName role.Name, tenantID tenant.ID) error {
	return r.manager.AddLink(sub.String(), roleKey(roleName.String()), tenantID.String())
}

func (r *casbinRoleResolver) addInheritance(child, parent role.Name, tenantID tenant.ID) error {
	return r.manager.AddLink(roleKey(child.String()), roleKey(parent.String()), tenantID.String())
}

func (r *casbinRoleResolver) DirectRoles(sub subject.Ref, tenantID tenant.ID) ([]role.Name, error) {
	if r == nil || r.manager == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "authorization role resolver is unavailable")
	}
	keys, err := r.manager.GetRoles(sub.String(), tenantID.String())
	if err != nil {
		return nil, err
	}
	return decodeRoleNames(keys)
}

func (r *casbinRoleResolver) EffectiveRoles(sub subject.Ref, tenantID tenant.ID) ([]role.Name, error) {
	if r == nil || r.manager == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "authorization role resolver is unavailable")
	}
	keys, err := r.manager.GetImplicitRoles(sub.String(), tenantID.String())
	if err != nil {
		return nil, err
	}
	return decodeRoleNames(keys)
}

func decodeRoleNames(keys []string) ([]role.Name, error) {
	values := make([]role.Name, 0, len(keys))
	for _, key := range keys {
		if !strings.HasPrefix(key, "role:") {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid authorization role key: %s", key)
		}
		name, err := role.NewName(strings.TrimPrefix(key, "role:"))
		if err != nil {
			return nil, err
		}
		values = append(values, name)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].String() < values[j].String() })
	return values, nil
}
