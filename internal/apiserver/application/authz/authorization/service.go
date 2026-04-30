package authorization

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authzDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

type DecisionEngine interface {
	Check(ctx context.Context, request authzDomain.AuthorizationRequest) (authzDomain.AuthorizationDecision, error)
}

type SnapshotStore interface {
	RoleNamesForSubject(ctx context.Context, subject authzDomain.Subject, tenantID string) ([]string, error)
	PermissionsForSubject(ctx context.Context, subject authzDomain.Subject, tenantID string) ([]authzDomain.Permission, error)
}

type Checker struct {
	engine DecisionEngine
}

func NewChecker(engine DecisionEngine) *Checker {
	return &Checker{engine: engine}
}

type CheckCommand struct {
	Subject     authzDomain.Subject
	TenantID    string
	ResourceKey string
	Action      string
	ObjectScope authzDomain.Scope
}

func (c *Checker) Check(ctx context.Context, cmd CheckCommand) (authzDomain.AuthorizationDecision, error) {
	if c == nil || c.engine == nil {
		return authzDomain.AuthorizationDecision{}, perrors.WithCode(code.ErrInternalServerError, "authorization engine not available")
	}
	scope := cmd.ObjectScope.Normalized()
	request, err := authzDomain.NewAuthorizationRequest(cmd.Subject, cmd.TenantID, cmd.ResourceKey, cmd.Action, authzDomain.WithObjectScope(scope))
	if err != nil {
		return authzDomain.AuthorizationDecision{}, err
	}
	return c.engine.Check(ctx, request)
}

type SnapshotReader struct {
	store     SnapshotStore
	versions  policyDomain.Repository
	projector SnapshotProjector
}

func NewSnapshotReader(store SnapshotStore, versions policyDomain.Repository) *SnapshotReader {
	return &SnapshotReader{store: store, versions: versions, projector: SnapshotProjector{}}
}

type SnapshotQuery struct {
	Subject  authzDomain.Subject
	TenantID string
	AppName  string
}

type PermissionEntry struct {
	ResourceKey string
	Action      string
	Scope       authzDomain.Scope
}

type Snapshot struct {
	Roles        []string
	Permissions  []PermissionEntry
	AuthzVersion int64
}

func (r *SnapshotReader) Read(ctx context.Context, query SnapshotQuery) (*Snapshot, error) {
	if r == nil || r.store == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "authorization snapshot store not available")
	}
	if r.versions == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "authorization version repository not available")
	}
	if query.Subject.Type == "" || strings.TrimSpace(query.Subject.ID) == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "subject is required")
	}
	if strings.TrimSpace(query.TenantID) == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "tenant id is required")
	}
	if strings.TrimSpace(query.AppName) == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "app name is required")
	}

	roleNames, err := r.store.RoleNamesForSubject(ctx, query.Subject, query.TenantID)
	if err != nil {
		return nil, err
	}
	permissions, err := r.store.PermissionsForSubject(ctx, query.Subject, query.TenantID)
	if err != nil {
		return nil, err
	}
	version, err := r.versions.GetOrCreate(ctx, query.TenantID)
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		Roles:        r.projector.RolesForApp(roleNames, query.AppName),
		Permissions:  r.projector.PermissionsForApp(permissions, query.AppName),
		AuthzVersion: version.Version,
	}, nil
}

type SnapshotProjector struct{}

func (SnapshotProjector) RolesForApp(roleNames []string, appName string) []string {
	seen := make(map[string]struct{}, len(roleNames))
	roles := make([]string, 0, len(roleNames))
	appPrefix := appName + ":"
	for _, roleName := range roleNames {
		if !strings.HasPrefix(roleName, appPrefix) {
			continue
		}
		if _, exists := seen[roleName]; exists {
			continue
		}
		seen[roleName] = struct{}{}
		roles = append(roles, roleName)
	}
	return roles
}

func (SnapshotProjector) PermissionsForApp(permissions []authzDomain.Permission, appName string) []PermissionEntry {
	seen := make(map[string]struct{}, len(permissions))
	result := make([]PermissionEntry, 0, len(permissions))
	appPrefix := appName + ":"
	for _, permission := range permissions {
		if !strings.HasPrefix(permission.ResourceKey, appPrefix) {
			continue
		}
		key := permission.ResourceKey + "\x00" + permission.Action + "\x00" + permission.Scope.Normalized().String()
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, PermissionEntry{
			ResourceKey: permission.ResourceKey,
			Action:      permission.Action,
			Scope:       permission.Scope,
		})
	}
	return result
}
