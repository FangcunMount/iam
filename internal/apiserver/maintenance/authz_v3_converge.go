package maintenance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policychange"
	authzuow "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/uow"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	assignmentrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/assignment"
	permissiongrantrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/permissiongrant"
	resourcerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/resource"
	rolerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	roleinheritancerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/roleinheritance"
	dbmysql "github.com/FangcunMount/iam/v3/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

const (
	AuthzV3ConvergeLockName = "iam.authz.v3.converge.v1"

	AuthzV3ConvergeConfirmation = "APPLY_AUTHZ_V3_CONVERGENCE"
	convergeActor               = "iam-maintenance:authz-v3-converge"
	missingSubjectFingerprint   = "dc38094e57858e16"
)

var (
	sourceCounts = AuthzV3Counts{Roles: 11, Resources: 25, Assignments: 137, Inheritances: 6, Grants: 105}
	targetCounts = AuthzV3Counts{Roles: 9, Resources: 27, Assignments: 135, Inheritances: 8, Grants: 100}

	missingSubjectRoles = []string{
		"qs:content_manager",
		"qs:evaluation_plan_manager",
		"qs:evaluator",
		"qs:staff",
	}
)

type AuthzV3Counts struct {
	Roles        int `json:"roles"`
	Resources    int `json:"resources"`
	Assignments  int `json:"assignments"`
	Inheritances int `json:"role_inheritances"`
	Grants       int `json:"permission_grants"`
}

func (c AuthzV3Counts) Equal(other AuthzV3Counts) bool {
	return c == other
}

type AuthzV3ConvergeSummary struct {
	Phase                      string           `json:"phase"`
	Counts                     AuthzV3Counts    `json:"counts"`
	ExpectedSourceCounts       AuthzV3Counts    `json:"expected_source_counts"`
	ExpectedTargetCounts       AuthzV3Counts    `json:"expected_target_counts"`
	StateHash                  string           `json:"state_hash"`
	MissingSubjectFingerprints []string         `json:"missing_subject_fingerprints,omitempty"`
	PlannedChanges             []string         `json:"planned_changes,omitempty"`
	Pending                    []string         `json:"pending,omitempty"`
	Blockers                   []string         `json:"blockers,omitempty"`
	FailureCode                string           `json:"failure_code,omitempty"`
	PolicyVersions             map[string]int64 `json:"policy_versions,omitempty"`
	Complete                   bool             `json:"complete"`
}

type AuthzV3ConvergePlan struct {
	Summary          AuthzV3ConvergeSummary
	missingSubjectID string
}

type convergeState struct {
	roles        []*rolerepo.RolePO
	resources    []*resourcerepo.ResourcePO
	assignments  []*assignmentrepo.AssignmentPO
	inheritances []*roleinheritancerepo.InheritancePO
	grants       []*permissiongrantrepo.GrantPO
	users        map[string]convergeUserState
}

type convergeUserState struct {
	Active bool
}

type grantSpec struct {
	Tenant      string
	Role        string
	Resource    string
	Action      string
	Constraints string
}

type inheritanceSpec struct {
	Tenant string
	Role   string
	Parent string
}

type resourceSpec struct {
	Key         string
	DisplayName string
	Actions     []string
	Description string
}

var sourceGrantRemovals = []grantSpec{
	unconditionalGrant("fangcun", "tenant_admin", "qs:answersheet:collection:answersheets", "read"),
	unconditionalGrant("fangcun", "tenant_admin", "qs:answersheet:collection:answersheets", "list"),
	unconditionalGrant("fangcun", "tenant_admin", "qs:answersheet:collection:answersheets", "statistics"),
	unconditionalGrant("fangcun", "qs:evaluator", "qs:actor:collection:testees", "read"),
	unconditionalGrant("fangcun", "qs:evaluator", "qs:actor:collection:testees", "list"),
	unconditionalGrant("fangcun", "tenant_admin", "iam:authz:action:check", "check"),
	unconditionalGrant("fangcun", "tenant_admin", "iam:authz:collection:policies", "read"),
	unconditionalGrant("fangcun", "tenant_admin", "iam:authz:collection:policies", "write"),
	unconditionalGrant("fangcun", "tenant_admin", "iam:authz:collection:policies", "delete"),
	unconditionalGrant("fangcun", "tenant_admin", "iam:authz:collection:assignments", "read"),
	unconditionalGrant("fangcun", "tenant_admin", "iam:authz:collection:assignments", "delete"),
	unconditionalGrant("fangcun", "super_admin", "iam:identity:collection:profiles", "search_by_mobile"),
}

var targetGrantAdditions = []grantSpec{
	unconditionalGrant("fangcun", "tenant_admin", "iam:authz:collection:permission_grants", "list"),
	unconditionalGrant("fangcun", "tenant_admin", "iam:authz:collection:permission_grants", "create"),
	unconditionalGrant("fangcun", "tenant_admin", "iam:authz:collection:permission_grants", "revoke"),
	unconditionalGrant("fangcun", "tenant_admin", "iam:authz:collection:assignments", "list"),
	unconditionalGrant("fangcun", "tenant_admin", "iam:authz:collection:role_inheritances", "list"),
	unconditionalGrant("fangcun", "tenant_admin", "iam:authz:collection:role_inheritances", "grant"),
	unconditionalGrant("fangcun", "tenant_admin", "iam:authz:collection:role_inheritances", "revoke"),
}

var targetInheritanceAdditions = []inheritanceSpec{
	{Tenant: "fangcun", Role: "super_admin", Parent: "tenant_admin"},
	{Tenant: "fangcun", Role: "super_admin", Parent: "qs:admin"},
}

var sourceInheritances = []inheritanceSpec{
	{Tenant: "fangcun", Role: "tenant_admin", Parent: "user"},
	{Tenant: "fangcun", Role: "qs:admin", Parent: "qs:content_manager"},
	{Tenant: "fangcun", Role: "qs:admin", Parent: "qs:evaluator"},
	{Tenant: "fangcun", Role: "qs:admin", Parent: "qs:evaluation_plan_manager"},
	{Tenant: "fangcun", Role: "qs:evaluator", Parent: "qs:staff"},
	{Tenant: "fangcun", Role: "qs:evaluation_plan_manager", Parent: "qs:staff"},
}

var targetRoleKeys = []string{
	"platform\x00super_admin",
	"fangcun\x00super_admin",
	"fangcun\x00tenant_admin",
	"fangcun\x00user",
	"fangcun\x00qs:admin",
	"fangcun\x00qs:content_manager",
	"fangcun\x00qs:evaluator",
	"fangcun\x00qs:staff",
	"fangcun\x00qs:evaluation_plan_manager",
}

var targetResourceKeys = []string{
	"iam:authn:collection:jwks",
	"iam:authn:collection:login_identities",
	"iam:authn:collection:sessions",
	"iam:authz:collection:assignments",
	"iam:authz:collection:permission_grants",
	"iam:authz:collection:resources",
	"iam:authz:collection:role_inheritances",
	"iam:authz:collection:roles",
	"iam:identity:collection:profile-links",
	"iam:identity:collection:profiles",
	"iam:identity:collection:users",
	"iam:identity:instance:profile",
	"iam:idp:collection:wechat_apps",
	"iam:ops:collection:cache_governance",
	"qs:actor:collection:staff",
	"qs:actor:collection:testees",
	"qs:answersheet:collection:answersheets",
	"qs:code:collection:codes",
	"qs:evaluation:collection:assessments",
	"qs:evaluation:collection:reports",
	"qs:modelcatalog:collection:norm_tables",
	"qs:plan:collection:evaluation_plans",
	"qs:plan_task:collection:evaluation_plan_tasks",
	"qs:questionnaire:collection:questionnaires",
	"qs:scale:collection:scales",
	"qs:statistics:collection:statistics_jobs",
	"qs:statistics:collection:system_statistics",
}

var targetResources = []resourceSpec{
	{
		Key:         "iam:authz:collection:role_inheritances",
		DisplayName: "角色继承",
		Actions:     []string{"list", "grant", "revoke"},
		Description: "角色继承关系管理",
	},
	{
		Key:         "iam:authn:collection:sessions",
		DisplayName: "会话管理",
		Actions:     []string{"revoke", "revoke_by_login_identity", "revoke_by_user"},
		Description: "认证会话撤销管理",
	},
	{
		Key:         "iam:ops:collection:cache_governance",
		DisplayName: "缓存治理",
		Actions:     []string{"read"},
		Description: "缓存目录与运行状态只读治理",
	},
}

func unconditionalGrant(tenantID, roleName, resourceKey, action string) grantSpec {
	encoded, err := constraint.Empty().CanonicalJSON()
	if err != nil {
		panic(err)
	}
	return grantSpec{Tenant: tenantID, Role: roleName, Resource: resourceKey, Action: action, Constraints: string(encoded)}
}

func AnalyzeAuthzV3Convergence(ctx context.Context, db *gorm.DB) (*AuthzV3ConvergePlan, error) {
	if db == nil {
		return nil, fmt.Errorf("authorization database is required")
	}
	var plan *AuthzV3ConvergePlan
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		plan, err = analyzeAuthzV3ConvergenceDB(ctx, tx)
		return err
	})
	return plan, err
}

func analyzeAuthzV3ConvergenceDB(ctx context.Context, db *gorm.DB) (*AuthzV3ConvergePlan, error) {
	state, err := loadConvergeState(ctx, db)
	if err != nil {
		return nil, err
	}
	stateHash, err := hashConvergeState(state)
	if err != nil {
		return nil, err
	}
	plan := &AuthzV3ConvergePlan{Summary: AuthzV3ConvergeSummary{
		Counts:               state.counts(),
		ExpectedSourceCounts: sourceCounts,
		ExpectedTargetCounts: targetCounts,
		StateHash:            stateHash,
		PolicyVersions:       map[string]int64{},
	}}
	plan.Summary.PolicyVersions, err = loadPolicyVersions(ctx, db)
	if err != nil {
		return nil, err
	}
	validateReferenceIntegrity(state, plan)
	if len(plan.Summary.Blockers) > 0 {
		finishConvergePlan(plan)
		return plan, nil
	}

	switch {
	case plan.Summary.Counts.Equal(sourceCounts):
		plan.Summary.Phase = "source"
		validateSourceState(state, plan)
		if len(plan.Summary.Blockers) == 0 {
			plan.Summary.PlannedChanges = plannedConvergenceChanges()
		}
	case isCatalogConvergedCounts(plan.Summary.Counts):
		plan.Summary.Phase = "catalog_converged"
		validateTargetState(state, plan)
		if len(plan.Summary.Blockers) == 0 && plan.Summary.Counts.Assignments == targetCounts.Assignments-2 {
			plan.Summary.Pending = append(plan.Summary.Pending, "synthetic_matrix_subjects_not_provisioned")
		}
		if len(plan.Summary.Blockers) == 0 && plan.Summary.Counts.Assignments == targetCounts.Assignments {
			plan.Summary.Phase = "final"
			plan.Summary.Complete = true
		}
	default:
		plan.Summary.Phase = "unknown"
		plan.block("active_counts_do_not_match_approved_source_or_target")
	}
	finishConvergePlan(plan)
	return plan, nil
}

func (p *AuthzV3ConvergePlan) block(value string) {
	if p == nil || strings.TrimSpace(value) == "" {
		return
	}
	for _, existing := range p.Summary.Blockers {
		if existing == value {
			return
		}
	}
	p.Summary.Blockers = append(p.Summary.Blockers, value)
}

func finishConvergePlan(plan *AuthzV3ConvergePlan) {
	sort.Strings(plan.Summary.Blockers)
	sort.Strings(plan.Summary.Pending)
	sort.Strings(plan.Summary.PlannedChanges)
	sort.Strings(plan.Summary.MissingSubjectFingerprints)
}

func isCatalogConvergedCounts(counts AuthzV3Counts) bool {
	return counts.Roles == targetCounts.Roles &&
		counts.Resources == targetCounts.Resources &&
		(counts.Assignments == targetCounts.Assignments-2 || counts.Assignments == targetCounts.Assignments) &&
		counts.Inheritances == targetCounts.Inheritances &&
		counts.Grants == targetCounts.Grants
}

func (s *convergeState) counts() AuthzV3Counts {
	return AuthzV3Counts{
		Roles:        len(s.roles),
		Resources:    len(s.resources),
		Assignments:  len(s.assignments),
		Inheritances: len(s.inheritances),
		Grants:       len(s.grants),
	}
}

func loadConvergeState(ctx context.Context, db *gorm.DB) (*convergeState, error) {
	state := &convergeState{users: map[string]convergeUserState{}}
	queries := []struct {
		value any
		where string
	}{
		{&state.roles, "deleted_at IS NULL"},
		{&state.resources, "deleted_at IS NULL"},
		{&state.assignments, "deleted_at IS NULL"},
		{&state.inheritances, "revoked_at IS NULL AND deleted_at IS NULL"},
		{&state.grants, "revoked_at IS NULL AND deleted_at IS NULL"},
	}
	for _, query := range queries {
		if err := db.WithContext(ctx).Where(query.where).Find(query.value).Error; err != nil {
			return nil, err
		}
	}
	type userRow struct {
		SubjectID string  `gorm:"column:subject_id"`
		Status    int     `gorm:"column:status"`
		DeletedAt *string `gorm:"column:deleted_at"`
	}
	var users []userRow
	if err := db.WithContext(ctx).Table("users").
		Select("CAST(id AS CHAR) AS subject_id, status, CAST(deleted_at AS CHAR) AS deleted_at").
		Find(&users).Error; err != nil {
		return nil, err
	}
	for _, row := range users {
		state.users[row.SubjectID] = convergeUserState{Active: row.Status == 1 && row.DeletedAt == nil}
	}
	return state, nil
}

func loadPolicyVersions(ctx context.Context, db *gorm.DB) (map[string]int64, error) {
	type row struct {
		TenantID string `gorm:"column:tenant_id"`
		Version  int64  `gorm:"column:policy_version"`
	}
	var rows []row
	if err := db.WithContext(ctx).Table("authz_policy_versions").
		Select("tenant_id, MAX(policy_version) AS policy_version").
		Where("deleted_at IS NULL").Group("tenant_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, item := range rows {
		result[item.TenantID] = item.Version
	}
	return result, nil
}

func validateReferenceIntegrity(state *convergeState, plan *AuthzV3ConvergePlan) {
	rolesByID := make(map[uint64]*rolerepo.RolePO, len(state.roles))
	resourcesByID := make(map[uint64]*resourcerepo.ResourcePO, len(state.resources))
	for _, item := range state.roles {
		rolesByID[item.ID.Uint64()] = item
	}
	for _, item := range state.resources {
		resourcesByID[item.ID.Uint64()] = item
	}
	for _, item := range state.assignments {
		role := rolesByID[item.RoleID]
		if role == nil || role.TenantID != item.TenantID {
			plan.block("assignment_role_reference_invalid")
		}
	}
	for _, item := range state.inheritances {
		child, parent := rolesByID[item.RoleID], rolesByID[item.InheritedRoleID]
		if child == nil || parent == nil || child.TenantID != item.TenantID || parent.TenantID != item.TenantID {
			plan.block("role_inheritance_reference_invalid")
		}
	}
	for _, item := range state.grants {
		role := rolesByID[item.RoleID]
		if role == nil || role.TenantID != item.TenantID {
			plan.block("permission_grant_role_reference_invalid")
			continue
		}
		if item.ResourceID != nil {
			catalog := resourcesByID[*item.ResourceID]
			if catalog == nil || catalog.Key != item.ResourcePattern {
				plan.block("permission_grant_resource_reference_invalid")
			}
		}
		if _, err := (permissiongrantrepo.Mapper{}).ToBO(item); err != nil {
			plan.block("permission_grant_canonical_hash_invalid")
		}
	}
	if inheritanceCycleFromState(state) {
		plan.block("role_inheritance_cycle")
	}
}

func inheritanceCycleFromState(state *convergeState) bool {
	edges := make([]*roleinheritance.Inheritance, 0, len(state.inheritances))
	mapper := roleinheritancerepo.Mapper{}
	for _, row := range state.inheritances {
		item, err := mapper.ToBO(row)
		if err != nil {
			return true
		}
		edges = append(edges, item)
	}
	for _, edge := range edges {
		others := make([]*roleinheritance.Inheritance, 0, len(edges)-1)
		for _, candidate := range edges {
			if candidate.ID != edge.ID {
				others = append(others, candidate)
			}
		}
		if roleinheritance.WouldCreateCycle(others, edge.RoleID, edge.InheritedRoleID) {
			return true
		}
	}
	return false
}

func validateSourceState(state *convergeState, plan *AuthzV3ConvergePlan) {
	roles := rolesByKey(state)
	resources := resourcesByKey(state)
	if !emptyRetirableRole(state, roles["platform\x00platform:admin"]) ||
		!emptyRetirableRole(state, roles["platform\x00iam:admin"]) {
		plan.block("retired_platform_roles_not_empty_or_missing")
	}
	for _, key := range targetRoleKeys {
		if roles[key] == nil {
			plan.block("source_role_manifest_mismatch")
		}
	}
	assertResourceActions(plan, resources, "iam:authz:collection:policies", []string{"read", "write", "delete"})
	assertResourceActions(plan, resources, "iam:authz:collection:assignments", []string{"read", "grant", "revoke", "delete"})
	assertResourceActions(plan, resources, "iam:authz:action:check", []string{"check"})
	for _, target := range targetResources {
		if resources[target.Key] != nil {
			plan.block("target_resource_already_present_in_source")
		}
	}
	if resources["iam:authz:collection:permission_grants"] != nil {
		plan.block("target_permission_grants_resource_already_present_in_source")
	}
	for _, spec := range sourceGrantRemovals {
		if countGrantSpec(state, spec) != 1 {
			plan.block("source_grant_diff_not_exact")
		}
	}
	for _, spec := range targetGrantAdditions {
		if countGrantSpec(state, spec) != 0 {
			plan.block("target_grant_already_present_in_source")
		}
	}
	for _, spec := range targetInheritanceAdditions {
		if countInheritanceSpec(state, spec) != 0 {
			plan.block("target_inheritance_already_present_in_source")
		}
	}
	for _, spec := range sourceInheritances {
		if countInheritanceSpec(state, spec) != 1 {
			plan.block("source_role_inheritance_manifest_mismatch")
		}
	}
	findMissingSubject(state, plan)
}

func validateTargetState(state *convergeState, plan *AuthzV3ConvergePlan) {
	roles := rolesByKey(state)
	resources := resourcesByKey(state)
	if roles["platform\x00platform:admin"] != nil || roles["platform\x00iam:admin"] != nil {
		plan.block("retired_platform_roles_still_active")
	}
	if roles["fangcun\x00super_admin"] == nil || roles["fangcun\x00tenant_admin"] == nil || roles["fangcun\x00qs:admin"] == nil {
		plan.block("target_role_graph_role_missing")
	}
	for _, key := range targetRoleKeys {
		if roles[key] == nil {
			plan.block("target_role_manifest_mismatch")
		}
	}
	assertResourceActions(plan, resources, "iam:authz:collection:permission_grants", []string{"list", "create", "revoke"})
	assertResourceActions(plan, resources, "iam:authz:collection:assignments", []string{"list", "grant", "revoke"})
	assertResourceActions(plan, resources, "iam:authz:collection:role_inheritances", []string{"list", "grant", "revoke"})
	assertResourceActions(plan, resources, "iam:authn:collection:sessions", []string{"revoke", "revoke_by_login_identity", "revoke_by_user"})
	assertResourceActions(plan, resources, "iam:ops:collection:cache_governance", []string{"read"})
	assertResourceIncludesAction(plan, resources, "iam:idp:collection:wechat_apps", "list", "update", "enable", "disable")
	assertResourceIncludesAction(plan, resources, "qs:evaluation:collection:reports", "audit")
	for _, key := range targetResourceKeys {
		if resources[key] == nil {
			plan.block("target_resource_manifest_mismatch")
		}
	}
	if resources["iam:authz:collection:policies"] != nil || resources["iam:authz:action:check"] != nil {
		plan.block("retired_authz_resources_still_active")
	}
	for _, spec := range sourceGrantRemovals {
		if countGrantSpec(state, spec) != 0 {
			plan.block("retired_grant_still_active")
		}
	}
	for _, spec := range targetGrantAdditions {
		if countGrantSpec(state, spec) != 1 {
			plan.block("target_grant_missing_or_duplicated")
		}
	}
	for _, spec := range targetInheritanceAdditions {
		if countInheritanceSpec(state, spec) != 1 {
			plan.block("target_inheritance_missing_or_duplicated")
		}
	}
	for _, spec := range sourceInheritances {
		if countInheritanceSpec(state, spec) != 1 {
			plan.block("target_role_inheritance_manifest_mismatch")
		}
	}
	if activeMissingSubjectAssignments(state) != 0 {
		plan.block("missing_iam_user_assignments_still_active")
	}
}

func findMissingSubject(state *convergeState, plan *AuthzV3ConvergePlan) {
	findMissingSubjectWithFingerprint(state, plan, missingSubjectFingerprint)
}

func findMissingSubjectWithFingerprint(state *convergeState, plan *AuthzV3ConvergePlan, expectedFingerprint string) {
	rolesByID := make(map[uint64]*rolerepo.RolePO, len(state.roles))
	for _, item := range state.roles {
		rolesByID[item.ID.Uint64()] = item
	}
	roleSets := make(map[string][]string)
	for _, item := range state.assignments {
		if item.SubjectType != "user" || item.TenantID != "fangcun" {
			continue
		}
		if user, exists := state.users[item.SubjectID]; exists && user.Active {
			continue
		}
		role := rolesByID[item.RoleID]
		if role != nil {
			roleSets[item.SubjectID] = append(roleSets[item.SubjectID], role.Name)
		}
	}
	for subjectID, roleNames := range roleSets {
		sort.Strings(roleNames)
		if equalStrings(roleNames, missingSubjectRoles) && fingerprintSubject(subjectID) == expectedFingerprint {
			if plan.missingSubjectID != "" {
				plan.block("multiple_missing_subjects_match_expected_roles")
				continue
			}
			plan.missingSubjectID = subjectID
			plan.Summary.MissingSubjectFingerprints = []string{expectedFingerprint}
		}
	}
	if plan.missingSubjectID == "" {
		plan.block("expected_missing_subject_assignment_set_not_found")
	}
	if len(roleSets) != 1 {
		plan.block("unexpected_missing_or_inactive_user_assignments")
	}
}

func activeMissingSubjectAssignments(state *convergeState) int {
	count := 0
	for _, item := range state.assignments {
		if item.SubjectType != "user" {
			continue
		}
		if user, exists := state.users[item.SubjectID]; !exists || !user.Active {
			count++
		}
	}
	return count
}

func emptyRetirableRole(state *convergeState, role *rolerepo.RolePO) bool {
	if role == nil {
		return false
	}
	id := role.ID.Uint64()
	for _, item := range state.assignments {
		if item.RoleID == id {
			return false
		}
	}
	for _, item := range state.grants {
		if item.RoleID == id {
			return false
		}
	}
	for _, item := range state.inheritances {
		if item.RoleID == id || item.InheritedRoleID == id {
			return false
		}
	}
	return true
}

func countGrantSpec(state *convergeState, spec grantSpec) int {
	roles := rolesByID(state)
	count := 0
	for _, item := range state.grants {
		role := roles[item.RoleID]
		if role == nil {
			continue
		}
		canonical, err := canonicalConstraint(item.ConstraintSet)
		if err != nil {
			continue
		}
		if item.TenantID == spec.Tenant && role.Name == spec.Role && item.ResourcePattern == spec.Resource &&
			item.Action == spec.Action && canonical == spec.Constraints {
			count++
		}
	}
	return count
}

func countInheritanceSpec(state *convergeState, spec inheritanceSpec) int {
	roles := rolesByID(state)
	count := 0
	for _, item := range state.inheritances {
		child, parent := roles[item.RoleID], roles[item.InheritedRoleID]
		if child != nil && parent != nil && item.TenantID == spec.Tenant && child.Name == spec.Role && parent.Name == spec.Parent {
			count++
		}
	}
	return count
}

func assertResourceActions(plan *AuthzV3ConvergePlan, resources map[string]*resourcerepo.ResourcePO, key string, expected []string) {
	item := resources[key]
	if item == nil {
		plan.block("resource_manifest_missing_" + strings.ReplaceAll(key, ":", "_"))
		return
	}
	actions, err := canonicalActions(item.Actions)
	if err != nil || !equalStrings(actions, sortedCopy(expected)) {
		plan.block("resource_action_manifest_mismatch_" + strings.ReplaceAll(key, ":", "_"))
	}
}

func assertResourceIncludesAction(plan *AuthzV3ConvergePlan, resources map[string]*resourcerepo.ResourcePO, key string, expected ...string) {
	item := resources[key]
	if item == nil {
		plan.block("resource_manifest_missing_" + strings.ReplaceAll(key, ":", "_"))
		return
	}
	actions, err := canonicalActions(item.Actions)
	if err != nil {
		plan.block("resource_action_manifest_mismatch_" + strings.ReplaceAll(key, ":", "_"))
		return
	}
	for _, action := range expected {
		if !contains(actions, action) {
			plan.block("resource_action_manifest_mismatch_" + strings.ReplaceAll(key, ":", "_"))
		}
	}
}

func rolesByKey(state *convergeState) map[string]*rolerepo.RolePO {
	result := make(map[string]*rolerepo.RolePO, len(state.roles))
	for _, item := range state.roles {
		result[item.TenantID+"\x00"+item.Name] = item
	}
	return result
}

func rolesByID(state *convergeState) map[uint64]*rolerepo.RolePO {
	result := make(map[uint64]*rolerepo.RolePO, len(state.roles))
	for _, item := range state.roles {
		result[item.ID.Uint64()] = item
	}
	return result
}

func resourcesByKey(state *convergeState) map[string]*resourcerepo.ResourcePO {
	result := make(map[string]*resourcerepo.ResourcePO, len(state.resources))
	for _, item := range state.resources {
		result[item.Key] = item
	}
	return result
}

func plannedConvergenceChanges() []string {
	return []string{
		"assignments.revoke_missing_user_exact_four",
		"grants.replace_legacy_admin_actions",
		"grants.revoke_redundant_qs_and_profile_permissions",
		"inheritances.add_fangcun_super_admin_parents",
		"resources.add_sessions_role_inheritances_cache_governance",
		"resources.retire_check_and_rename_policies",
		"roles.retire_empty_platform_admin_roles",
		"roles.update_evaluator_description",
	}
}

func hashConvergeState(state *convergeState) (string, error) {
	lines := make([]string, 0, len(state.roles)+len(state.resources)+len(state.assignments)+len(state.inheritances)+len(state.grants))
	roles := rolesByID(state)
	resources := make(map[uint64]*resourcerepo.ResourcePO, len(state.resources))
	for _, item := range state.resources {
		resources[item.ID.Uint64()] = item
		actions, err := canonicalActions(item.Actions)
		if err != nil {
			return "", err
		}
		attributeSchema, err := canonicalJSON(item.AttributeSchema)
		if err != nil {
			return "", err
		}
		lines = append(lines, fmt.Sprintf("resource|%s|%s|%s|%s|%s|%s|%s", item.Key, item.DisplayName, item.AppName, item.Domain, item.Type, strings.Join(actions, ","), attributeSchema))
	}
	for _, item := range state.roles {
		lines = append(lines, fmt.Sprintf("role|%s|%s|%s|%d|%s", item.TenantID, item.Name, item.DisplayName, item.IsSystem, item.Description))
	}
	for _, item := range state.assignments {
		role := roles[item.RoleID]
		if role == nil {
			return "", fmt.Errorf("assignment role is missing")
		}
		lines = append(lines, fmt.Sprintf("assignment|%s|%s|%s|%s", item.TenantID, item.SubjectType, fingerprintSubject(item.SubjectID), role.Name))
	}
	for _, item := range state.inheritances {
		child, parent := roles[item.RoleID], roles[item.InheritedRoleID]
		if child == nil || parent == nil {
			return "", fmt.Errorf("inheritance role is missing")
		}
		lines = append(lines, fmt.Sprintf("inheritance|%s|%s|%s", item.TenantID, child.Name, parent.Name))
	}
	for _, item := range state.grants {
		role := roles[item.RoleID]
		if role == nil {
			return "", fmt.Errorf("grant role is missing")
		}
		constraints, err := canonicalConstraint(item.ConstraintSet)
		if err != nil {
			return "", err
		}
		catalogKey := "wildcard"
		if item.ResourceID != nil {
			catalog := resources[*item.ResourceID]
			if catalog == nil {
				return "", fmt.Errorf("grant resource is missing")
			}
			catalogKey = catalog.Key
		}
		lines = append(lines, fmt.Sprintf("grant|%s|%s|%s|%s|%s|%s", item.TenantID, role.Name, catalogKey, item.ResourcePattern, item.Action, constraints))
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

func canonicalActions(raw string) ([]string, error) {
	var actions []string
	if err := json.Unmarshal([]byte(raw), &actions); err != nil {
		return nil, err
	}
	actions = sortedCopy(actions)
	return actions, nil
}

func canonicalConstraint(raw string) (string, error) {
	set, err := constraint.ParseJSON([]byte(raw))
	if err != nil {
		return "", err
	}
	encoded, err := set.CanonicalJSON()
	return string(encoded), err
}

func canonicalJSON(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "null" {
		return "null", nil
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(value)
	return string(encoded), err
}

func fingerprintSubject(subjectID string) string {
	sum := sha256.Sum256([]byte(subjectID))
	return hex.EncodeToString(sum[:8])
}

func sortedCopy(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func contains(values []string, target string) bool {
	index := sort.SearchStrings(values, target)
	return index < len(values) && values[index] == target
}

func ApplyAuthzV3Convergence(ctx context.Context, db *gorm.DB, uow authzuow.UnitOfWork, expectedSourceHash string) (*AuthzV3ConvergePlan, error) {
	if db == nil || uow == nil {
		return nil, fmt.Errorf("authorization database and unit of work are required")
	}
	expectedSourceHash = strings.TrimSpace(expectedSourceHash)
	if len(expectedSourceHash) != sha256.Size*2 {
		return nil, fmt.Errorf("expected source hash is required")
	}
	var applied *AuthzV3ConvergePlan
	err := uow.WithinTx(ctx, func(txCtx context.Context, repos authzuow.TxRepositories) error {
		tx, err := dbmysql.RequireTx(txCtx)
		if err != nil {
			return err
		}
		plan, err := analyzeAuthzV3ConvergenceDB(txCtx, tx)
		if err != nil {
			return err
		}
		applied = plan
		if len(plan.Summary.Blockers) > 0 {
			return fmt.Errorf("authorization convergence source is blocked")
		}
		if plan.Summary.Phase != "source" {
			if plan.Summary.Phase == "catalog_converged" || plan.Summary.Phase == "final" {
				applied = plan
				return nil
			}
			return fmt.Errorf("authorization convergence source phase is invalid")
		}
		if plan.Summary.StateHash != expectedSourceHash {
			plan.block("approved_source_hash_mismatch")
			finishConvergePlan(plan)
			return fmt.Errorf("authorization convergence source hash changed")
		}
		if err := applyConvergenceMutations(txCtx, repos, plan); err != nil {
			return err
		}
		for _, tenantID := range []string{"fangcun", "platform"} {
			version, err := repos.PolicyVersions.Increment(txCtx, tenantID, convergeActor, "AuthZ V3 semantic convergence")
			if err != nil {
				plan.block("apply_policy_version_increment_failed_" + tenantID)
				return err
			}
			if err := policychange.StagePolicyVersionChanged(txCtx, repos.Events, tenantID, version); err != nil {
				plan.block("apply_policy_version_event_failed_" + tenantID)
				return err
			}
		}
		applied, err = analyzeAuthzV3ConvergenceDB(txCtx, tx)
		if err != nil {
			return err
		}
		if len(applied.Summary.Blockers) > 0 || applied.Summary.Phase != "catalog_converged" {
			return fmt.Errorf("authorization convergence target verification failed")
		}
		return nil
	})
	return applied, err
}

func applyConvergenceMutations(ctx context.Context, repos authzuow.TxRepositories, plan *AuthzV3ConvergePlan) (err error) {
	step := "resolve_roles"
	defer func() {
		if err != nil {
			plan.block("apply_" + step + "_failed")
			plan.Summary.FailureCode = convergenceFailureCode(err)
		}
	}()
	roles := make(map[string]meta.ID)
	for _, key := range []struct{ tenant, name string }{
		{"platform", "platform:admin"}, {"platform", "iam:admin"}, {"fangcun", "super_admin"},
		{"fangcun", "tenant_admin"}, {"fangcun", "qs:admin"}, {"fangcun", "qs:evaluator"},
	} {
		item, err := repos.Roles.FindByName(ctx, key.tenant, key.name)
		if err != nil {
			return err
		}
		roles[key.tenant+"\x00"+key.name] = item.ID
	}

	step = "revoke_source_grants"
	// Convergence only mutates active policy facts. Avoid restoring revoked history
	// here because rows created under older canonicalization rules may carry stale
	// grant keys even though they cannot participate in authorization decisions.
	activeGrantsByTenant := make(map[string][]*permissiongrant.Grant)
	for specIndex, spec := range sourceGrantRemovals {
		step = fmt.Sprintf("revoke_source_grant_%02d_resolve_role", specIndex)
		roleID := roles[spec.Tenant+"\x00"+spec.Role]
		if roleID.IsZero() {
			role, err := repos.Roles.FindByName(ctx, spec.Tenant, spec.Role)
			if err != nil {
				return err
			}
			roleID = role.ID
			roles[spec.Tenant+"\x00"+spec.Role] = roleID
		}
		grants, loaded := activeGrantsByTenant[spec.Tenant]
		if !loaded {
			step = fmt.Sprintf("revoke_source_grant_%02d_load_active", specIndex)
			grants, err = repos.PermissionGrants.ListActiveByTenant(ctx, spec.Tenant)
			if err != nil {
				return err
			}
			activeGrantsByTenant[spec.Tenant] = grants
		}
		matched := 0
		for _, item := range grants {
			step = fmt.Sprintf("revoke_source_grant_%02d_canonicalize", specIndex)
			constraints, err := item.CanonicalConstraintJSON()
			if err != nil {
				return err
			}
			if item.RoleID == roleID && item.ResourcePatternString() == spec.Resource && item.ActionString() == spec.Action && string(constraints) == spec.Constraints {
				matched++
				step = fmt.Sprintf("revoke_source_grant_%02d_update", specIndex)
				if err := repos.PermissionGrants.Revoke(ctx, item.ID); err != nil {
					return err
				}
			}
		}
		step = fmt.Sprintf("revoke_source_grant_%02d_match", specIndex)
		if matched != 1 {
			return fmt.Errorf("source grant changed during convergence")
		}
	}

	step = "rename_permission_grants_resource"
	resources := make(map[string]*resource.Resource)
	policies, err := repos.Resources.FindByKey(ctx, "iam:authz:collection:policies")
	if err != nil {
		return err
	}
	permissionGrantsResource, err := resource.NewResource(
		"iam:authz:collection:permission_grants", []string{"list", "create", "revoke"},
		resource.WithID(policies.ID), resource.WithDisplayName("权限授权"), resource.WithDescription("角色的资源、动作与对象条件授权"),
	)
	if err != nil {
		return err
	}
	if err := repos.Resources.Update(ctx, &permissionGrantsResource); err != nil {
		return err
	}
	resources[permissionGrantsResource.KeyString()] = &permissionGrantsResource

	step = "normalize_assignment_resource"
	if err := replaceResourceActions(ctx, repos, resources, "iam:authz:collection:assignments", []string{"list", "grant", "revoke"}); err != nil {
		return err
	}
	step = "extend_wechat_resource"
	if err := appendResourceActions(ctx, repos, resources, "iam:idp:collection:wechat_apps", "list", "update", "enable", "disable"); err != nil {
		return err
	}
	step = "extend_report_resource"
	if err := appendResourceActions(ctx, repos, resources, "qs:evaluation:collection:reports", "audit"); err != nil {
		return err
	}
	step = "create_resources"
	for _, spec := range targetResources {
		item, err := resource.NewResource(spec.Key, spec.Actions, resource.WithDisplayName(spec.DisplayName), resource.WithDescription(spec.Description))
		if err != nil {
			return err
		}
		if err := repos.Resources.Create(ctx, &item); err != nil {
			return err
		}
		resources[item.KeyString()] = &item
	}

	step = "create_grants"
	for _, spec := range targetGrantAdditions {
		roleID := roles[spec.Tenant+"\x00"+spec.Role]
		catalog := resources[spec.Resource]
		if catalog == nil {
			catalog, err = repos.Resources.FindByKey(ctx, spec.Resource)
			if err != nil {
				return err
			}
		}
		item, err := permissiongrant.New(roleID, spec.Tenant, catalog.ID, spec.Resource, spec.Action, constraint.Empty(), convergeActor)
		if err != nil {
			return err
		}
		if err := item.ValidateAgainst(*catalog); err != nil {
			return err
		}
		if err := repos.PermissionGrants.Create(ctx, &item); err != nil {
			return err
		}
	}

	step = "create_inheritances"
	for _, spec := range targetInheritanceAdditions {
		childID := roles[spec.Tenant+"\x00"+spec.Role]
		parentID := roles[spec.Tenant+"\x00"+spec.Parent]
		if parentID.IsZero() {
			parent, err := repos.Roles.FindByName(ctx, spec.Tenant, spec.Parent)
			if err != nil {
				return err
			}
			parentID = parent.ID
			roles[spec.Tenant+"\x00"+spec.Parent] = parentID
		}
		current, err := repos.RoleInheritances.ListActiveByTenant(ctx, spec.Tenant)
		if err != nil {
			return err
		}
		if roleinheritance.WouldCreateCycle(current, childID, parentID) {
			return fmt.Errorf("target inheritance would create a cycle")
		}
		item, err := roleinheritance.New(childID, parentID, spec.Tenant, convergeActor)
		if err != nil {
			return err
		}
		if err := repos.RoleInheritances.Create(ctx, &item); err != nil {
			return err
		}
	}

	step = "update_evaluator_description"
	evaluator, err := repos.Roles.FindByName(ctx, "fangcun", "qs:evaluator")
	if err != nil {
		return err
	}
	evaluator.ChangeDescription("测评执行、批量评估及仅 adhoc 测评重试")
	if err := repos.Roles.Update(ctx, evaluator); err != nil {
		return err
	}
	step = "retire_check_resource"
	checkResource, err := repos.Resources.FindByKey(ctx, "iam:authz:action:check")
	if err != nil {
		return err
	}
	if checkResource == nil {
		return fmt.Errorf("validated check resource disappeared during convergence")
	}
	if err := repos.Resources.Delete(ctx, checkResource.ID); err != nil {
		return err
	}
	step = "retire_empty_roles"
	for _, roleName := range []string{"platform:admin", "iam:admin"} {
		if err := repos.Roles.Delete(ctx, roles["platform\x00"+roleName]); err != nil {
			return err
		}
	}

	step = "revoke_missing_subject_assignments"
	subjectID, err := meta.ParseID(plan.missingSubjectID)
	if err != nil || subjectID.IsZero() {
		return fmt.Errorf("validated missing subject id is invalid")
	}
	assignments, err := repos.Assignments.ListBySubject(ctx, "user", subjectID, "fangcun")
	if err != nil {
		return err
	}
	roleNamesByID := make(map[meta.ID]string)
	for _, name := range missingSubjectRoles {
		item, err := repos.Roles.FindByName(ctx, "fangcun", name)
		if err != nil {
			return err
		}
		roleNamesByID[item.ID] = name
	}
	removed := 0
	for _, assignment := range assignments {
		if _, ok := roleNamesByID[assignment.RoleID]; ok {
			removed++
			if err := repos.Assignments.Delete(ctx, assignment.ID); err != nil {
				return err
			}
		}
	}
	if removed != len(missingSubjectRoles) {
		return fmt.Errorf("missing subject assignment set changed during convergence")
	}
	return nil
}

func convergenceFailureCode(err error) string {
	var mysqlErr *mysqldriver.MySQLError
	if errors.As(err, &mysqlErr) {
		return fmt.Sprintf("mysql_%d", mysqlErr.Number)
	}
	return "non_mysql_error"
}

func replaceResourceActions(ctx context.Context, repos authzuow.TxRepositories, cache map[string]*resource.Resource, key string, actions []string) error {
	item, err := repos.Resources.FindByKey(ctx, key)
	if err != nil {
		return err
	}
	if err := item.ChangeCatalog(actions); err != nil {
		return err
	}
	if err := repos.Resources.Update(ctx, item); err != nil {
		return err
	}
	cache[key] = item
	return nil
}

func appendResourceActions(ctx context.Context, repos authzuow.TxRepositories, cache map[string]*resource.Resource, key string, actions ...string) error {
	item, err := repos.Resources.FindByKey(ctx, key)
	if err != nil {
		return err
	}
	merged := append(item.ActionStrings(), actions...)
	if err := item.ChangeCatalog(merged); err != nil {
		return err
	}
	if err := repos.Resources.Update(ctx, item); err != nil {
		return err
	}
	cache[key] = item
	return nil
}

func VerifyAuthzV3Convergence(ctx context.Context, db *gorm.DB) (*AuthzV3ConvergePlan, error) {
	plan, err := AnalyzeAuthzV3Convergence(ctx, db)
	if err != nil {
		return nil, err
	}
	if len(plan.Summary.Blockers) > 0 {
		return plan, fmt.Errorf("authorization convergence verification is blocked")
	}
	if !plan.Summary.Complete {
		return plan, fmt.Errorf("authorization convergence awaits synthetic matrix subjects")
	}
	return plan, nil
}
