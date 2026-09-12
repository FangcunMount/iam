package runtime

import (
	"sort"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authorizationapp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/authorization"
	authorizationdomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v5/internal/pkg/code"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
)

type Snapshot struct {
	assignmentsBySubject map[string][]AssignmentRecord
	roleFacts            map[meta.ID]authorizationapp.AssignmentRoleFact
	verifiedAt           time.Time // proof belongs to this immutable publication
	roles                authorizationdomain.RoleResolver
	roleNames            map[meta.ID]role.Name
	grantsByRole         map[meta.ID][]*permissiongrant.Grant
	resources            map[string]*resource.Resource
	version              int64
	loadedAt             time.Time
}

func BuildSnapshot(dataset Dataset, loadedAt time.Time) (*Snapshot, error) {
	if loadedAt.IsZero() {
		loadedAt = time.Now()
	}
	roleByID := make(map[meta.ID]RoleRecord, len(dataset.Roles))
	uniqueNames := make(map[string]struct{}, len(dataset.Roles))
	for _, record := range dataset.Roles {
		record.Name = strings.TrimSpace(record.Name)
		if err := record.ManagementProtection.Validate(); err != nil {
			return nil, err
		}
		if record.ID.IsZero() || record.Name == "" {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid role record in authorization runtime dataset")
		}
		uniqueName := record.Name
		if _, exists := uniqueNames[uniqueName]; exists {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "duplicate runtime role name: %s", record.Name)
		}
		if _, exists := roleByID[record.ID]; exists {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "duplicate runtime role id: %s", record.ID.String())
		}
		uniqueNames[uniqueName] = struct{}{}
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
		owned := catalogResource.Clone()
		catalogResource = &owned
		resources[key] = catalogResource
		resourcesByID[catalogResource.ID.Uint64()] = catalogResource
	}

	assignmentsBySubject := map[string][]AssignmentRecord{}
	directRoleBuilder := newDirectRoleBuilder()
	for _, assignment := range dataset.Assignments {
		_, ok := roleByID[assignment.RoleID]
		if !ok {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "角色分配引用了不存在的角色")
		}
		sub, err := subject.ParseRef(assignment.SubjectKey)
		if err != nil {
			return nil, err
		}
		if assignment.Scope != nil {
			value := *assignment.Scope
			if value.IsZero() {
				return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid assignment scope in snapshot")
			}
			assignment.Scope = &value
		}
		assignmentsBySubject[sub.String()] = append(assignmentsBySubject[sub.String()], assignment)
		directRoleBuilder.addAssignment(sub, assignment.RoleID)
	}
	roleResolver := directRoleBuilder.build()

	grantsByRole := make(map[meta.ID][]*permissiongrant.Grant)
	for _, grant := range dataset.Grants {
		if grant == nil || !grant.IsActive() {
			continue
		}
		roleRecord, ok := roleByID[grant.RoleID]
		if !ok {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "权限授予引用了不存在的角色")
		}
		if err := (role.Role{ManagementProtection: roleRecord.ManagementProtection}).ValidateGrant(grant.ResourceKey, grant.Action); err != nil {
			return nil, err
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

		owned := grant.Clone()
		grant = &owned
		grantsByRole[grant.RoleID] = append(grantsByRole[grant.RoleID], grant)
	}
	for name := range grantsByRole {
		sort.Slice(grantsByRole[name], func(i, j int) bool { return grantsByRole[name][i].ID < grantsByRole[name][j].ID })
	}
	roleNames := make(map[meta.ID]role.Name, len(roleByID))
	roleFacts := make(map[meta.ID]authorizationapp.AssignmentRoleFact, len(roleByID))
	for id, r := range roleByID {
		name, err := role.NewName(r.Name)
		if err != nil {
			return nil, err
		}
		roleNames[id] = name
		roleFacts[id] = authorizationapp.AssignmentRoleFact{RoleID: id.String(), RoleName: name.String(), ManagementProtection: string(r.ManagementProtection)}
	}

	return &Snapshot{
		assignmentsBySubject: assignmentsBySubject, roleFacts: roleFacts, roles: roleResolver, roleNames: roleNames, grantsByRole: grantsByRole, resources: resources,
		version: dataset.Version, loadedAt: loadedAt,
	}, nil
}

func (s *Snapshot) evaluationContext(request authorizationdomain.Request) (authorizationdomain.EvaluationContext, error) {
	if s == nil || s.roles == nil {
		return authorizationdomain.EvaluationContext{}, perrors.WithCode(code.ErrInternalServerError, "authorization runtime snapshot is unavailable")
	}
	roles, err := s.roles.EffectiveRoles(request.Subject)
	if err != nil {
		return authorizationdomain.EvaluationContext{}, err
	}

	return authorizationdomain.EvaluationContext{
		EffectiveRoles: roles,
		RoleNames:      s.roleNames,
		GrantsByRole:   s.grantsByRole,
		Resource:       s.resources[request.ResourceKey.String()],
		PolicyVersion:  s.version,
	}, nil
}

func (s *Snapshot) SubjectSnapshot(sub subject.Ref, appName string) (authorizationapp.SubjectSnapshot, error) {
	effectiveRoles, err := s.roles.EffectiveRoles(sub)
	if err != nil {
		return authorizationapp.SubjectSnapshot{}, err
	}
	directRoles, err := s.roles.DirectRoles(sub)
	if err != nil {
		return authorizationapp.SubjectSnapshot{}, err
	}
	facts := make([]authorizationapp.AssignmentRoleFact, 0, len(directRoles))
	for _, id := range directRoles {
		facts = append(facts, s.roleFacts[id])
	}
	sort.Slice(facts, func(i, j int) bool { return facts[i].RoleID < facts[j].RoleID })
	modeByPermission := make(map[string]authorizationapp.AuthorizationMode)
	for _, roleName := range effectiveRoles {
		for _, grant := range s.grantsByRole[roleName] {
			resourceApp, ok := resource.AppNameFromKey(grant.ResourceKeyString())
			if !ok || resourceApp != appName {
				continue
			}
			key := grant.ResourceKeyString() + "\x00" + grant.ActionString()
			modeByPermission[key] = authorizationapp.ModeUnconditional
		}
	}
	permissions := make([]authorizationapp.PermissionEntry, 0, len(modeByPermission))
	for key, mode := range modeByPermission {
		parts := strings.SplitN(key, "\x00", 2)
		permissions = append(permissions, authorizationapp.PermissionEntry{Resource: parts[0], Action: parts[1], Mode: mode, Scopes: s.permissionScopes(sub, parts[0], parts[1])})
	}
	sort.Slice(permissions, func(i, j int) bool {
		if permissions[i].Resource == permissions[j].Resource {
			return permissions[i].Action < permissions[j].Action
		}
		return permissions[i].Resource < permissions[j].Resource
	})
	return authorizationapp.SubjectSnapshot{
		AssignmentFacts: facts, AssignmentFactsComplete: true, AssignmentScopes: s.assignmentScopeFacts(sub),
		DirectRoles:    appScopedRoleNames(s.names(directRoles), appName),
		EffectiveRoles: appScopedRoleNames(s.names(effectiveRoles), appName),
		Permissions:    permissions,
		PolicyVersion:  s.version,
	}, nil
}

func (s *Snapshot) effectiveRoleNamesForSubject(sub subject.Ref) ([]string, error) {
	roles, err := s.roles.EffectiveRoles(sub)
	if err != nil {
		return nil, err
	}
	roleNames := make([]string, 0, len(roles))
	for _, roleName := range roles {
		roleNames = append(roleNames, s.roleNames[roleName].String())
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

func (s *Snapshot) Version() int64 { return s.version }

func (s *Snapshot) names(ids []meta.ID) []role.Name {
	out := make([]role.Name, 0, len(ids))
	for _, id := range ids {
		out = append(out, s.roleNames[id])
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (s *Snapshot) assignmentScopeFacts(sub subject.Ref) []authorizationapp.AssignmentScopeFact {
	result := make([]authorizationapp.AssignmentScopeFact, 0, len(s.assignmentsBySubject[sub.String()]))
	for _, a := range s.assignmentsBySubject[sub.String()] {
		fact := authorizationapp.AssignmentScopeFact{AssignmentID: a.ID.String(), Role: s.roleFacts[a.RoleID]}
		if a.Scope != nil {
			value := *a.Scope
			fact.Scope = &value
		}
		result = append(result, fact)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].AssignmentID < result[j].AssignmentID })
	return result
}
