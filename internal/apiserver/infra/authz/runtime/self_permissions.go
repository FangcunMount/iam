package runtime

import (
	"context"
	"sort"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v5/internal/pkg/code"

	authzapp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/authorization"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/subject"
)

// PermissionEntriesForSubject exposes only the requested authenticated subject's
// direct grants for the self-profile transport, including global wildcard grants.
func (r *Runtime) PermissionEntriesForSubject(ctx context.Context, sub subject.Ref) ([]authzapp.PermissionEntry, error) {
	snapshot := r.current.Load()
	if snapshot == nil || !r.freshSnapshot(snapshot) {
		return nil, perrors.WithCode(code.ErrAuthorizationPolicyUnavailable, "authorization runtime policy unavailable")
	}
	roles, err := snapshot.roles.DirectRoles(sub)
	if err != nil {
		return nil, err
	}
	result := []authzapp.PermissionEntry{}
	seen := map[string]bool{}
	for _, id := range roles {
		for _, g := range snapshot.grantsByRole[id] {
			key := g.ResourceKeyString() + "\x00" + g.ActionString()
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, authzapp.PermissionEntry{Resource: g.ResourceKeyString(), Action: g.ActionString(), Mode: authzapp.ModeUnconditional, Scopes: snapshot.permissionScopes(sub, g.ResourceKeyString(), g.ActionString())})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Resource == result[j].Resource {
			return result[i].Action < result[j].Action
		}
		return result[i].Resource < result[j].Resource
	})
	return result, nil
}
