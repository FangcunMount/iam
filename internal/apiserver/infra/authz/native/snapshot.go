package native

import (
	"fmt"
	"sort"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authorizationapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	authorizationdomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

const maxRoleHierarchyLevel = 32

type Snapshot struct {
	roles        authorizationdomain.RoleResolver
	grantsByRole map[tenant.ID]map[role.Name][]*permissiongrant.Grant
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

	roleResolver := newCasbinRoleResolver(maxRoleHierarchyLevel)
	for _, assignment := range dataset.Assignments {
		roleRecord, ok := roleByID[assignment.RoleID]
		if !ok || roleRecord.TenantID != assignment.TenantID {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "assignment references an unknown or cross-tenant role")
		}
		sub, err := parseSubjectKey(assignment.SubjectKey)
		if err != nil {
			return nil, err
		}
		roleName, err := role.NewName(roleRecord.Name)
		if err != nil {
			return nil, err
		}
		tenantID, err := tenant.NewID(assignment.TenantID)
		if err != nil {
			return nil, err
		}
		if err := roleResolver.addAssignment(sub, roleName, tenantID); err != nil {
			return nil, fmt.Errorf("add assignment role link: %w", err)
		}
	}
	if err := validateInheritanceGraph(dataset.Inheritances, roleByID); err != nil {
		return nil, err
	}
	for _, inheritance := range dataset.Inheritances {
		roleRecord := roleByID[inheritance.RoleID]
		inherited := roleByID[inheritance.InheritedRoleID]
		childName, err := role.NewName(roleRecord.Name)
		if err != nil {
			return nil, err
		}
		parentName, err := role.NewName(inherited.Name)
		if err != nil {
			return nil, err
		}
		tenantID, err := tenant.NewID(inheritance.TenantID)
		if err != nil {
			return nil, err
		}
		if err := roleResolver.addInheritance(childName, parentName, tenantID); err != nil {
			return nil, fmt.Errorf("add role inheritance link: %w", err)
		}
	}

	grantsByRole := make(map[tenant.ID]map[role.Name][]*permissiongrant.Grant)
	for _, grant := range dataset.Grants {
		if grant == nil || !grant.IsActive() {
			continue
		}
		roleRecord, ok := roleByID[grant.RoleID]
		if !ok || roleRecord.TenantID != grant.TenantIDString() {
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
		tenantID := grant.TenantID
		roleName, err := role.NewName(roleRecord.Name)
		if err != nil {
			return nil, err
		}
		if grantsByRole[tenantID] == nil {
			grantsByRole[tenantID] = make(map[role.Name][]*permissiongrant.Grant)
		}
		grantsByRole[tenantID][roleName] = append(grantsByRole[tenantID][roleName], grant)
	}
	for _, grantsForTenant := range grantsByRole {
		for roleName := range grantsForTenant {
			sort.Slice(grantsForTenant[roleName], func(i, j int) bool {
				return grantsForTenant[roleName][i].ID.Uint64() < grantsForTenant[roleName][j].ID.Uint64()
			})
		}
	}

	versions := make(map[string]int64, len(dataset.Versions))
	for tenantID, version := range dataset.Versions {
		versions[strings.TrimSpace(tenantID)] = version
	}
	return &Snapshot{
		roles: roleResolver, grantsByRole: grantsByRole, resources: resources,
		versions: versions, loadedAt: loadedAt,
	}, nil
}

func (s *Snapshot) EvaluationContext(request authorizationdomain.Request) (authorizationdomain.EvaluationContext, error) {
	if s == nil || s.roles == nil {
		return authorizationdomain.EvaluationContext{}, perrors.WithCode(code.ErrInternalServerError, "authorization runtime snapshot is unavailable")
	}
	tenantID := request.TenantIDString()
	roles, err := s.roles.EffectiveRoles(request.Subject, request.TenantID)
	if err != nil {
		return authorizationdomain.EvaluationContext{}, err
	}

	return authorizationdomain.EvaluationContext{
		EffectiveRoles: roles,
		GrantsByRole:   s.grantsByRole[request.TenantID],
		Resource:       s.resources[request.ResourceKey.String()],
		PolicyVersion:  s.versions[tenantID],
	}, nil
}

func (s *Snapshot) SubjectSnapshot(sub subject.Ref, tenantID, appName string) (authorizationapp.SubjectSnapshot, error) {
	tenantValue, err := tenant.NewID(tenantID)
	if err != nil {
		return authorizationapp.SubjectSnapshot{}, err
	}
	effectiveRoles, err := s.roles.EffectiveRoles(sub, tenantValue)
	if err != nil {
		return authorizationapp.SubjectSnapshot{}, err
	}
	directRoles, err := s.roles.DirectRoles(sub, tenantValue)
	if err != nil {
		return authorizationapp.SubjectSnapshot{}, err
	}
	modeByPermission := make(map[string]authorizationapp.AuthorizationMode)
	for _, roleName := range effectiveRoles {
		for _, grant := range s.grantsByRole[tenantValue][roleName] {
			resourceApp, ok := resource.AppNameFromKey(grant.ResourcePatternString())
			if !ok || resourceApp != appName {
				continue
			}
			key := grant.ResourcePatternString() + "\x00" + grant.ActionString()
			mode := authorizationapp.ModeObjectCheckRequired
			if !grant.IsConditional() {
				mode = authorizationapp.ModeUnconditional
			}
			if current, exists := modeByPermission[key]; !exists || current == authorizationapp.ModeObjectCheckRequired && mode == authorizationapp.ModeUnconditional {
				modeByPermission[key] = mode
			}
		}
	}
	permissions := make([]authorizationapp.PermissionEntry, 0, len(modeByPermission))
	for key, mode := range modeByPermission {
		parts := strings.SplitN(key, "\x00", 2)
		permissions = append(permissions, authorizationapp.PermissionEntry{Resource: parts[0], Action: parts[1], Mode: mode})
	}
	sort.Slice(permissions, func(i, j int) bool {
		if permissions[i].Resource == permissions[j].Resource {
			return permissions[i].Action < permissions[j].Action
		}
		return permissions[i].Resource < permissions[j].Resource
	})
	return authorizationapp.SubjectSnapshot{
		DirectRoles:    appScopedRoleNames(directRoles, appName),
		EffectiveRoles: appScopedRoleNames(effectiveRoles, appName),
		Permissions:    permissions,
		PolicyVersion:  s.versions[tenantID],
	}, nil
}

func (s *Snapshot) DirectRoleKeys(sub subject.Ref, tenantID string) ([]string, error) {
	tenantValue, err := tenant.NewID(tenantID)
	if err != nil {
		return nil, err
	}
	roles, err := s.roles.DirectRoles(sub, tenantValue)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(roles))
	for _, roleName := range roles {
		keys = append(keys, roleKey(roleName.String()))
	}
	return keys, nil
}

func (s *Snapshot) roleNamesForSubject(sub subject.Ref, tenantID string) ([]string, error) {
	tenantValue, err := tenant.NewID(tenantID)
	if err != nil {
		return nil, err
	}
	roles, err := s.roles.EffectiveRoles(sub, tenantValue)
	if err != nil {
		return nil, err
	}
	roleNames := make([]string, 0, len(roles))
	for _, roleName := range roles {
		roleNames = append(roleNames, roleName.String())
	}
	return roleNames, nil
}

func appScopedRoleNames(values []role.Name, appName string) []string {
	roles := make([]string, 0, len(values))
	for _, value := range values {
		if app, ok := value.App(); ok && app == appName {
			roles = append(roles, value.String())
		}
	}
	return uniqueSortedStrings(roles)
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
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
