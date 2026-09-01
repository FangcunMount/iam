package authorization

import (
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
)

// RoleResolver defines the authorization-domain capability for resolving a
// subject's direct and inherited roles inside one tenant boundary.
type RoleResolver interface {
	DirectRoles(subject.Ref, tenant.ID) ([]role.Name, error)
	EffectiveRoles(subject.Ref, tenant.ID) ([]role.Name, error)
}
