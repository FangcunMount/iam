package native

import (
	"fmt"
	"sort"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	authzruntime "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/runtime"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	defaultrolemanager "github.com/casbin/casbin/v2/rbac/default-role-manager"
)

const maxRoleHierarchyLevel = 32

type Snapshot struct {
	roles        *defaultrolemanager.RoleManager
	grantsByRole map[string][]*permissiongrant.Grant
	resources    map[string]*resource.Resource
	versions     map[string]int64
	loadedAt     time.Time
}

func BuildSnapshot(dataset Dataset, loadedAt time.Time) (*Snapshot, error) {
	if loadedAt.IsZero() {
		loadedAt = time.Now()
	}
	roleByID := make(map[meta.ID]RoleRecord, len(dataset.Roles))
	roleNames := make(map[string]struct{}, len(dataset.Roles))
	for _, record := range dataset.Roles {
		record.TenantID = strings.TrimSpace(record.TenantID)
		record.Name = strings.TrimSpace(record.Name)
		if record.ID.IsZero() || record.TenantID == "" || record.Name == "" {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid role record in authorization runtime dataset")
		}
		uniqueName := record.TenantID + "\x00" + record.Name
		if _, exists := roleNames[uniqueName]; exists {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "duplicate runtime role name: %s", record.Name)
		}
		if _, exists := roleByID[record.ID]; exists {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "duplicate runtime role id: %s", record.ID.String())
		}
		roleNames[uniqueName] = struct{}{}
		roleByID[record.ID] = record
	}

	resources := make(map[string]*resource.Resource, len(dataset.Resources))
	resourcesByID := make(map[uint64]*resource.Resource, len(dataset.Resources))
	for _, catalogResource := range dataset.Resources {
		if catalogResource == nil || catalogResource.ID.Uint64() == 0 {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid resource record in authorization runtime dataset")
		}
		key := catalogResource.KeyString()
		if _, exists := resources[key]; exists {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "duplicate runtime resource key: %s", key)
		}
		resources[key] = catalogResource
		resourcesByID[catalogResource.ID.Uint64()] = catalogResource
	}

	roleManager := defaultrolemanager.NewRoleManager(maxRoleHierarchyLevel)
	for _, assignment := range dataset.Assignments {
		role, ok := roleByID[assignment.RoleID]
		if !ok || role.TenantID != assignment.TenantID {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "assignment references an unknown or cross-tenant role")
		}
		if _, err := parseSubjectKey(assignment.SubjectKey); err != nil {
			return nil, err
		}
		if err := roleManager.AddLink(assignment.SubjectKey, roleKey(role.Name), assignment.TenantID); err != nil {
			return nil, fmt.Errorf("add assignment role link: %w", err)
		}
	}
	if err := validateInheritanceGraph(dataset.Inheritances, roleByID); err != nil {
		return nil, err
	}
	for _, inheritance := range dataset.Inheritances {
		role := roleByID[inheritance.RoleID]
		inherited := roleByID[inheritance.InheritedRoleID]
		if err := roleManager.AddLink(roleKey(role.Name), roleKey(inherited.Name), inheritance.TenantID); err != nil {
			return nil, fmt.Errorf("add role inheritance link: %w", err)
		}
	}

	grantsByRole := make(map[string][]*permissiongrant.Grant)
	for _, grant := range dataset.Grants {
		if grant == nil || !grant.IsActive() {
			continue
		}
		role, ok := roleByID[grant.RoleID]
		if !ok || role.TenantID != grant.TenantIDString() {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "permission grant references an unknown or cross-tenant role")
		}
		if grant.ResourceID.Uint64() != 0 {
			catalogResource, ok := resourcesByID[grant.ResourceID.Uint64()]
			if !ok {
				return nil, perrors.WithCode(code.ErrInvalidArgument, "permission grant references an unknown resource")
			}
			if err := grant.ValidateAgainst(*catalogResource); err != nil {
				return nil, err
			}
		}
		key := tenantRoleKey(grant.TenantIDString(), role.Name)
		grantsByRole[key] = append(grantsByRole[key], grant)
	}
	for key := range grantsByRole {
		sort.Slice(grantsByRole[key], func(i, j int) bool {
			return grantsByRole[key][i].ID.Uint64() < grantsByRole[key][j].ID.Uint64()
		})
	}

	versions := make(map[string]int64, len(dataset.Versions))
	for tenantID, version := range dataset.Versions {
		versions[strings.TrimSpace(tenantID)] = version
	}
	return &Snapshot{
		roles: roleManager, grantsByRole: grantsByRole, resources: resources,
		versions: versions, loadedAt: loadedAt,
	}, nil
}

func (s *Snapshot) Check(request authzruntime.Request) (authzruntime.Decision, error) {
	if s == nil || s.roles == nil {
		return authzruntime.Decision{}, perrors.WithCode(code.ErrInternalServerError, "authorization runtime snapshot is unavailable")
	}
	tenantID := request.TenantIDString()
	policyVersion := s.versions[tenantID]
	roles, err := s.roleNamesForSubject(request.Subject, tenantID)
	if err != nil {
		return authzruntime.Decision{}, err
	}
	if catalogResource := s.resources[request.ResourceKey.String()]; catalogResource != nil {
		if err := authzruntime.ValidateAttributes(catalogResource.AttributeSchema, request.Object.Attributes); err != nil {
			return authzruntime.Decision{}, err
		}
	} else if len(request.Object.Attributes) > 0 {
		return authzruntime.Decision{}, perrors.WithCode(code.ErrInvalidArgument, "object attributes require a registered resource")
	}

	missing := make([]string, 0)
	for _, roleName := range roles {
		for _, grant := range s.grantsByRole[tenantRoleKey(tenantID, roleName)] {
			if !grant.CoversResource(request.ResourceKey) || !grant.MatchesAction(request.Action) {
				continue
			}
			evaluation, err := grant.Evaluate(request.Object.Attributes)
			if err != nil {
				return authzruntime.Decision{}, err
			}
			if evaluation.Matched {
				return authzruntime.Allow(grant.ID, roleName, policyVersion, time.Now()), nil
			}
			missing = append(missing, evaluation.MissingAttributeKeys...)
		}
	}
	return authzruntime.Deny(policyVersion, missing, time.Now()), nil
}

func (s *Snapshot) SubjectSnapshot(sub subject.Ref, tenantID, appName string) (authzruntime.SubjectSnapshot, error) {
	roles, err := s.roleNamesForSubject(sub, tenantID)
	if err != nil {
		return authzruntime.SubjectSnapshot{}, err
	}
	modeByPermission := make(map[string]authzruntime.AuthorizationMode)
	for _, roleName := range roles {
		for _, grant := range s.grantsByRole[tenantRoleKey(tenantID, roleName)] {
			resourceApp, ok := resource.AppNameFromKey(grant.ResourcePatternString())
			if !ok || resourceApp != appName {
				continue
			}
			key := grant.ResourcePatternString() + "\x00" + grant.ActionString()
			mode := authzruntime.ModeObjectCheckRequired
			if !grant.IsConditional() {
				mode = authzruntime.ModeUnconditional
			}
			if current, exists := modeByPermission[key]; !exists || current == authzruntime.ModeObjectCheckRequired && mode == authzruntime.ModeUnconditional {
				modeByPermission[key] = mode
			}
		}
	}
	permissions := make([]authzruntime.PermissionEntry, 0, len(modeByPermission))
	for key, mode := range modeByPermission {
		parts := strings.SplitN(key, "\x00", 2)
		permissions = append(permissions, authzruntime.PermissionEntry{Resource: parts[0], Action: parts[1], Mode: mode})
	}
	sort.Slice(permissions, func(i, j int) bool {
		if permissions[i].Resource == permissions[j].Resource {
			return permissions[i].Action < permissions[j].Action
		}
		return permissions[i].Resource < permissions[j].Resource
	})
	appRoles := make([]string, 0, len(roles))
	for _, roleName := range roles {
		if app, ok := roleAppName(roleName); ok && app == appName {
			appRoles = append(appRoles, roleName)
		}
	}
	return authzruntime.SubjectSnapshot{Roles: appRoles, Permissions: permissions, PolicyVersion: s.versions[tenantID]}, nil
}

func (s *Snapshot) DirectRoleKeys(sub subject.Ref, tenantID string) ([]string, error) {
	roles, err := s.roles.GetRoles(sub.String(), tenantID)
	if err != nil {
		return nil, err
	}
	sort.Strings(roles)
	return roles, nil
}

func (s *Snapshot) roleNamesForSubject(sub subject.Ref, tenantID string) ([]string, error) {
	roleKeys, err := s.roles.GetImplicitRoles(sub.String(), tenantID)
	if err != nil {
		return nil, err
	}
	roleNames := make([]string, 0, len(roleKeys))
	for _, key := range roleKeys {
		roleNames = append(roleNames, strings.TrimPrefix(key, "role:"))
	}
	sort.Strings(roleNames)
	return roleNames, nil
}

func (s *Snapshot) LoadedAt() time.Time { return s.loadedAt }

func (s *Snapshot) Versions() map[string]int64 {
	copyVersions := make(map[string]int64, len(s.versions))
	for tenantID, version := range s.versions {
		copyVersions[tenantID] = version
	}
	return copyVersions
}

func validateInheritanceGraph(records []InheritanceRecord, roles map[meta.ID]RoleRecord) error {
	graph := make(map[string][]string)
	for _, record := range records {
		role, ok := roles[record.RoleID]
		inherited, inheritedOK := roles[record.InheritedRoleID]
		if !ok || !inheritedOK || role.TenantID != record.TenantID || inherited.TenantID != record.TenantID {
			return perrors.WithCode(code.ErrInvalidArgument, "role inheritance references an unknown or cross-tenant role")
		}
		if role.ID == inherited.ID {
			return perrors.WithCode(code.ErrInvalidArgument, "role inheritance contains a self-cycle")
		}
		graph[record.TenantID+"\x00"+role.Name] = append(graph[record.TenantID+"\x00"+role.Name], record.TenantID+"\x00"+inherited.Name)
	}
	visiting := make(map[string]bool)
	visited := make(map[string]bool)
	var visit func(string) bool
	visit = func(node string) bool {
		if visiting[node] {
			return true
		}
		if visited[node] {
			return false
		}
		visiting[node] = true
		for _, next := range graph[node] {
			if visit(next) {
				return true
			}
		}
		visiting[node] = false
		visited[node] = true
		return false
	}
	for node := range graph {
		if visit(node) {
			return perrors.WithCode(code.ErrInvalidArgument, "role inheritance contains a cycle")
		}
	}
	return nil
}

func roleKey(name string) string { return "role:" + name }

func tenantRoleKey(tenantID, roleName string) string { return tenantID + "\x00" + roleName }

func roleAppName(roleName string) (string, bool) {
	parts := strings.Split(roleName, ":")
	if len(parts) < 2 || parts[0] == "" {
		return "", false
	}
	return parts[0], true
}

func parseSubjectKey(value string) (subject.Ref, error) {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return subject.Ref{}, perrors.WithCode(code.ErrInvalidArgument, "subject must use <type>:<id>")
	}
	id, err := meta.ParseID(parts[1])
	if err != nil {
		return subject.Ref{}, perrors.WithCode(code.ErrInvalidArgument, "subject id is invalid")
	}
	return subject.NewRef(subject.Type(parts[0]), id)
}
