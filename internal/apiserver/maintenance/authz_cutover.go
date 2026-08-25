package maintenance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	permissiongrantrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/permissiongrant"
	policyrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/policy"
	resourcerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/resource"
	rolerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	bindingrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/rolebinding"
	roleinheritancerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/roleinheritance"
	"gorm.io/gorm"
)

const (
	AuthzCutoverLockName = "iam.authz.cutover.v1"
	assessmentKey        = "qs:evaluation:collection:assessments"
)

var safeAction = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type AuthzCutoverSummary struct {
	LegacyPolicyCount        int              `json:"legacy_policy_count"`
	LegacyGroupingCount      int              `json:"legacy_grouping_count"`
	ExpectedGrantCount       int              `json:"expected_grant_count"`
	ExpectedInheritanceCount int              `json:"expected_inheritance_count"`
	ExpectedHash             string           `json:"expected_hash"`
	Blockers                 []string         `json:"blockers,omitempty"`
	AppliedGrantCount        int              `json:"applied_grant_count,omitempty"`
	AppliedInheritanceCount  int              `json:"applied_inheritance_count,omitempty"`
	PolicyVersions           map[string]int64 `json:"policy_versions,omitempty"`
	Verified                 bool             `json:"verified"`
}

type AuthzCutoverPlan struct {
	Summary      AuthzCutoverSummary
	grants       []*permissiongrant.Grant
	inheritances []*roleinheritance.Inheritance
}

type legacyRule struct {
	ID    uint64  `gorm:"column:id"`
	PType string  `gorm:"column:ptype"`
	V0    *string `gorm:"column:v0"`
	V1    *string `gorm:"column:v1"`
	V2    *string `gorm:"column:v2"`
	V3    *string `gorm:"column:v3"`
	V4    *string `gorm:"column:v4"`
}

func AnalyzeAuthzCutover(ctx context.Context, db *gorm.DB) (*AuthzCutoverPlan, error) {
	if db == nil {
		return nil, fmt.Errorf("authorization cutover database is required")
	}
	plan := &AuthzCutoverPlan{}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return analyzeAuthzCutoverTx(tx, plan)
	})
	if err != nil {
		return nil, err
	}
	plan.Summary.ExpectedGrantCount = len(plan.grants)
	plan.Summary.ExpectedInheritanceCount = len(plan.inheritances)
	plan.Summary.ExpectedHash = planHash(plan.grants, plan.inheritances)
	sort.Strings(plan.Summary.Blockers)
	return plan, nil
}

func analyzeAuthzCutoverTx(db *gorm.DB, plan *AuthzCutoverPlan) error {
	var roleRows []*rolerepo.RolePO
	if err := db.Where("deleted_at IS NULL").Find(&roleRows).Error; err != nil {
		return err
	}
	roles := make(map[string]*rolerepo.RolePO, len(roleRows))
	rolesByID := make(map[uint64]*rolerepo.RolePO, len(roleRows))
	for _, role := range roleRows {
		roles[role.TenantID+"\x00"+role.Name] = role
		rolesByID[role.ID.Uint64()] = role
	}

	var resourceRows []*resourcerepo.ResourcePO
	if err := db.Where("deleted_at IS NULL").Find(&resourceRows).Error; err != nil {
		return err
	}
	resourceMapper := resourcerepo.NewMapper()
	resources := make(map[string]*resource.Resource, len(resourceRows))
	for _, row := range resourceRows {
		catalogResource, err := resourceMapper.ToBO(row)
		if err != nil {
			return err
		}
		resources[catalogResource.KeyString()] = catalogResource
	}

	var assignmentRows []*bindingrepo.BindingPO
	if err := db.Where("deleted_at IS NULL").Find(&assignmentRows).Error; err != nil {
		return err
	}
	assignmentFacts := make(map[string]struct{}, len(assignmentRows))
	for _, assignment := range assignmentRows {
		role := rolesByID[assignment.RoleID]
		if role == nil || role.TenantID != assignment.TenantID {
			plan.block("assignment_unknown_or_cross_tenant_role")
			continue
		}
		assignmentFacts[assignment.TenantID+"\x00"+assignment.SubjectType+":"+assignment.SubjectID+"\x00"+role.Name] = struct{}{}
	}

	var rules []legacyRule
	if err := db.Table("casbin_rule").Order("id ASC").Find(&rules).Error; err != nil {
		return err
	}
	grantKeys := make(map[string]struct{})
	inheritanceKeys := make(map[string]struct{})
	seenAssignmentFacts := make(map[string]struct{}, len(assignmentFacts))
	for _, rule := range rules {
		switch rule.PType {
		case "p":
			plan.Summary.LegacyPolicyCount++
			grants, blocker := convertLegacyPolicy(rule, roles, resources)
			if blocker != "" {
				plan.block(blocker)
				continue
			}
			for _, grant := range grants {
				if _, exists := grantKeys[grant.GrantKey]; exists {
					continue
				}
				grantKeys[grant.GrantKey] = struct{}{}
				plan.grants = append(plan.grants, grant)
			}
		case "g":
			plan.Summary.LegacyGroupingCount++
			inheritance, assignmentFact, blocker := convertLegacyGrouping(rule, roles, assignmentFacts)
			if blocker != "" {
				plan.block(blocker)
				continue
			}
			if assignmentFact != "" {
				seenAssignmentFacts[assignmentFact] = struct{}{}
			}
			if inheritance != nil {
				key := inheritance.TenantIDString() + "\x00" + inheritance.RoleID.String() + "\x00" + inheritance.InheritedRoleID.String()
				if _, exists := inheritanceKeys[key]; !exists {
					inheritanceKeys[key] = struct{}{}
					plan.inheritances = append(plan.inheritances, inheritance)
				}
			}
		default:
			plan.block("unknown_casbin_ptype")
		}
	}
	for assignmentFact := range assignmentFacts {
		if _, exists := seenAssignmentFacts[assignmentFact]; !exists {
			plan.block("assignment_grouping_mismatch")
		}
	}

	addPilotAssessmentRetryGrants(plan, roles, resources, grantKeys)
	if inheritanceCycle(plan.inheritances) {
		plan.block("role_inheritance_cycle")
	}
	return nil
}

func convertLegacyPolicy(rule legacyRule, roles map[string]*rolerepo.RolePO, resources map[string]*resource.Resource) ([]*permissiongrant.Grant, string) {
	roleName, ok := strings.CutPrefix(value(rule.V0), "role:")
	tenantID := strings.TrimSpace(value(rule.V1))
	resourcePattern := strings.TrimSpace(value(rule.V2))
	if !ok || tenantID == "" || resourcePattern == "" {
		return nil, "invalid_policy_shape"
	}
	role := roles[tenantID+"\x00"+roleName]
	if role == nil {
		return nil, "unknown_or_cross_tenant_policy_role"
	}
	pattern, err := resource.NewPattern(resourcePattern)
	if err != nil {
		return nil, "invalid_resource_pattern"
	}
	constraints, blocker := convertLegacyScope(value(rule.V4))
	if blocker != "" {
		return nil, blocker
	}
	actions, blocker := splitLegacyActions(value(rule.V3))
	if blocker != "" {
		return nil, blocker
	}
	catalogResource := resources[resourcePattern]
	wildcardResource := strings.Contains(resourcePattern, "*")
	if wildcardResource && !constraints.IsUnconditional() {
		return nil, "conditional_wildcard_policy"
	}
	if !wildcardResource && catalogResource == nil {
		return nil, "unknown_policy_resource"
	}
	if !constraints.IsUnconditional() {
		if catalogResource == nil || constraints.ValidateAgainst(catalogResource.AttributeSchema) != nil {
			return nil, "scope_attribute_schema_mismatch"
		}
	}
	grants := make([]*permissiongrant.Grant, 0, len(actions))
	for _, action := range actions {
		grantConstraints := constraints
		if roleName == "qs:evaluator" && resourcePattern == assessmentKey && action == "retry" {
			grantConstraints, err = constraint.New(constraint.Equal(attribute.ObjectOriginType, constraint.StringValue("adhoc")))
			if err != nil {
				return nil, "pilot_constraint_invalid"
			}
		}
		var grant permissiongrant.Grant
		if wildcardResource || action == permissiongrant.WildcardAction {
			if !grantConstraints.IsUnconditional() {
				return nil, "conditional_wildcard_policy"
			}
			grant, err = permissiongrant.NewSystem(role.ID, tenantID, resource.ResourceID{}, pattern.String(), action, grantConstraints, "authz-cutover")
		} else {
			if !catalogResource.HasAction(action) {
				return nil, "unknown_policy_action"
			}
			grant, err = permissiongrant.New(role.ID, tenantID, catalogResource.ID, pattern.String(), action, grantConstraints, "authz-cutover")
			if err == nil {
				err = grant.ValidateAgainst(*catalogResource)
			}
		}
		if err != nil {
			return nil, "invalid_permission_grant"
		}
		grants = append(grants, &grant)
	}
	return grants, ""
}

func convertLegacyScope(raw string) (constraint.Set, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "all:*" {
		return constraint.Empty(), ""
	}
	value, ok := strings.CutPrefix(raw, "origin:")
	if !ok || strings.TrimSpace(value) == "" {
		return constraint.Set{}, "invalid_scope"
	}
	set, err := constraint.New(constraint.Equal(attribute.ObjectOriginType, constraint.StringValue(strings.TrimSpace(value))))
	if err != nil {
		return constraint.Set{}, "invalid_scope"
	}
	return set, ""
}

func splitLegacyActions(raw string) ([]string, string) {
	raw = strings.TrimSpace(raw)
	if raw == ".*" {
		return []string{permissiongrant.WildcardAction}, ""
	}
	parts := strings.Split(raw, "|")
	actions := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !safeAction.MatchString(part) {
			return nil, "unsupported_action_expression"
		}
		if _, exists := seen[part]; exists {
			continue
		}
		seen[part] = struct{}{}
		actions = append(actions, part)
	}
	return actions, ""
}

func convertLegacyGrouping(rule legacyRule, roles map[string]*rolerepo.RolePO, assignments map[string]struct{}) (*roleinheritance.Inheritance, string, string) {
	tenantID := strings.TrimSpace(value(rule.V2))
	left := strings.TrimSpace(value(rule.V0))
	rightName, rightIsRole := strings.CutPrefix(strings.TrimSpace(value(rule.V1)), "role:")
	if tenantID == "" || !rightIsRole || roles[tenantID+"\x00"+rightName] == nil {
		return nil, "", "invalid_grouping_rule"
	}
	if leftName, leftIsRole := strings.CutPrefix(left, "role:"); leftIsRole {
		leftRole := roles[tenantID+"\x00"+leftName]
		rightRole := roles[tenantID+"\x00"+rightName]
		if leftRole == nil || rightRole == nil {
			return nil, "", "unknown_grouping_role"
		}
		inheritance, err := roleinheritance.New(leftRole.ID, rightRole.ID, tenantID, "authz-cutover")
		if err != nil {
			return nil, "", "invalid_role_inheritance"
		}
		return &inheritance, "", ""
	}
	parts := strings.SplitN(left, ":", 2)
	if len(parts) != 2 {
		return nil, "", "invalid_grouping_subject"
	}
	assignmentFact := tenantID + "\x00" + left + "\x00" + rightName
	if _, exists := assignments[assignmentFact]; !exists {
		return nil, "", "assignment_grouping_mismatch"
	}
	return nil, assignmentFact, ""
}

func addPilotAssessmentRetryGrants(plan *AuthzCutoverPlan, roles map[string]*rolerepo.RolePO, resources map[string]*resource.Resource, keys map[string]struct{}) {
	catalogResource := resources[assessmentKey]
	if catalogResource == nil || !catalogResource.HasAction("retry") {
		plan.block("pilot_assessment_resource_mapping_missing")
		return
	}
	pilotOrigins := map[string]string{
		"qs:evaluator":               "adhoc",
		"qs:evaluation_plan_manager": "plan",
	}
	foundRoles := make(map[string]bool, len(pilotOrigins))
	foundAdmin := false
	for _, role := range roles {
		if role.Name == "qs:admin" {
			foundAdmin = true
			grant, grantErr := permissiongrant.NewSystem(
				role.ID, role.TenantID, resource.ResourceID{}, "qs:*:*:*", permissiongrant.WildcardAction,
				constraint.Empty(), "authz-cutover",
			)
			if grantErr != nil {
				plan.block("pilot_admin_wildcard_invalid")
				continue
			}
			if _, exists := keys[grant.GrantKey]; !exists {
				keys[grant.GrantKey] = struct{}{}
				plan.grants = append(plan.grants, &grant)
			}
			continue
		}
		origin, isPilotRole := pilotOrigins[role.Name]
		if !isPilotRole {
			continue
		}
		foundRoles[role.Name] = true
		constraints, constraintErr := constraint.New(constraint.Equal(attribute.ObjectOriginType, constraint.StringValue(origin)))
		if constraintErr != nil {
			plan.block("pilot_constraint_invalid")
			continue
		}
		grant, grantErr := permissiongrant.New(role.ID, role.TenantID, catalogResource.ID, catalogResource.KeyString(), "retry", constraints, "authz-cutover")
		if grantErr != nil {
			plan.block("pilot_retry_grant_invalid")
			continue
		}
		if _, exists := keys[grant.GrantKey]; !exists {
			keys[grant.GrantKey] = struct{}{}
			plan.grants = append(plan.grants, &grant)
		}
	}
	for roleName := range pilotOrigins {
		if !foundRoles[roleName] {
			plan.block("pilot_" + strings.TrimPrefix(roleName, "qs:") + "_mapping_missing")
		}
	}
	if !foundAdmin {
		plan.block("pilot_admin_mapping_missing")
	}
}

func ApplyAuthzCutover(ctx context.Context, db *gorm.DB, plan *AuthzCutoverPlan) (AuthzCutoverSummary, error) {
	if plan == nil || len(plan.Summary.Blockers) > 0 {
		return AuthzCutoverSummary{}, fmt.Errorf("authorization cutover has unresolved blockers")
	}
	summary := plan.Summary
	summary.PolicyVersions = make(map[string]int64)
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		changedTenants := make(map[string]struct{})
		grantRepo := permissiongrantrepo.NewRepository(tx)
		var existingGrantKeys []string
		if err := tx.Table("authz_permission_grants").Where("deleted_at IS NULL AND revoked_at IS NULL").Pluck("grant_key", &existingGrantKeys).Error; err != nil {
			return err
		}
		existingGrants := make(map[string]struct{}, len(existingGrantKeys))
		for _, key := range existingGrantKeys {
			existingGrants[key] = struct{}{}
		}
		for _, planned := range plan.grants {
			if _, exists := existingGrants[planned.GrantKey]; exists {
				continue
			}
			grant := *planned
			if err := grantRepo.Create(ctx, &grant); err != nil {
				return err
			}
			summary.AppliedGrantCount++
			changedTenants[grant.TenantIDString()] = struct{}{}
		}

		inheritanceRepo := roleinheritancerepo.NewRepository(tx)
		for _, planned := range plan.inheritances {
			var count int64
			if err := tx.Table("authz_role_inheritances").Where(
				"tenant_id = ? AND role_id = ? AND inherited_role_id = ? AND deleted_at IS NULL AND revoked_at IS NULL",
				planned.TenantIDString(), planned.RoleID.Uint64(), planned.InheritedRoleID.Uint64(),
			).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			inheritance := *planned
			if err := inheritanceRepo.Create(ctx, &inheritance); err != nil {
				return err
			}
			summary.AppliedInheritanceCount++
			changedTenants[inheritance.TenantIDString()] = struct{}{}
		}

		if err := verifyAuthzCutoverTx(tx, plan); err != nil {
			return err
		}
		versionRepo := policyrepo.NewPolicyVersionRepository(tx)
		for tenantID := range changedTenants {
			version, err := versionRepo.Increment(ctx, tenantID, "authz-cutover", "native authorization cutover")
			if err != nil {
				return err
			}
			summary.PolicyVersions[tenantID] = version.Version
		}
		return persistCutoverVerification(tx, plan.Summary.ExpectedHash, time.Now())
	})
	if err != nil {
		return AuthzCutoverSummary{}, err
	}
	summary.Verified = true
	return summary, nil
}

func VerifyAuthzCutover(ctx context.Context, db *gorm.DB, plan *AuthzCutoverPlan) (AuthzCutoverSummary, error) {
	if plan == nil || len(plan.Summary.Blockers) > 0 {
		return AuthzCutoverSummary{}, fmt.Errorf("authorization cutover has unresolved blockers")
	}
	summary := plan.Summary
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return verifyAuthzCutoverTx(tx, plan) }); err != nil {
		return AuthzCutoverSummary{}, err
	}
	summary.Verified = true
	return summary, nil
}

type cutoverVerificationState struct {
	ID           uint64     `gorm:"column:id;primaryKey"`
	Status       string     `gorm:"column:status"`
	EvidenceHash string     `gorm:"column:evidence_hash"`
	VerifiedAt   *time.Time `gorm:"column:verified_at"`
}

func persistCutoverVerification(db *gorm.DB, evidenceHash string, verifiedAt time.Time) error {
	var current cutoverVerificationState
	err := db.Table("authz_cutover_state").Where("id = ?", 1).Take(&current).Error
	if err == nil && current.Status == "verified" && current.EvidenceHash == evidenceHash && current.VerifiedAt != nil {
		return nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if err == gorm.ErrRecordNotFound {
		return db.Table("authz_cutover_state").Create(&cutoverVerificationState{
			ID: 1, Status: "verified", EvidenceHash: evidenceHash, VerifiedAt: &verifiedAt,
		}).Error
	}
	return db.Table("authz_cutover_state").Where("id = ?", 1).Updates(map[string]any{
		"status": "verified", "evidence_hash": evidenceHash, "verified_at": verifiedAt,
	}).Error
}

func verifyAuthzCutoverTx(db *gorm.DB, plan *AuthzCutoverPlan) error {
	var grantKeys []string
	if err := db.Table("authz_permission_grants").Where("deleted_at IS NULL AND revoked_at IS NULL").Order("grant_key ASC").Pluck("grant_key", &grantKeys).Error; err != nil {
		return err
	}
	expectedKeys := make([]string, 0, len(plan.grants))
	for _, grant := range plan.grants {
		expectedKeys = append(expectedKeys, grant.GrantKey)
	}
	sort.Strings(expectedKeys)
	if strings.Join(grantKeys, "\x00") != strings.Join(expectedKeys, "\x00") {
		return fmt.Errorf("permission grant reconciliation mismatch")
	}
	type inheritanceKey struct {
		TenantID        string `gorm:"column:tenant_id"`
		RoleID          uint64 `gorm:"column:role_id"`
		InheritedRoleID uint64 `gorm:"column:inherited_role_id"`
	}
	var rows []inheritanceKey
	if err := db.Table("authz_role_inheritances").Select("tenant_id, role_id, inherited_role_id").Where("deleted_at IS NULL AND revoked_at IS NULL").Order("tenant_id, role_id, inherited_role_id").Scan(&rows).Error; err != nil {
		return err
	}
	actual := make([]string, 0, len(rows))
	for _, row := range rows {
		actual = append(actual, fmt.Sprintf("%s:%d:%d", row.TenantID, row.RoleID, row.InheritedRoleID))
	}
	expected := inheritancePlanKeys(plan.inheritances)
	if strings.Join(actual, "\x00") != strings.Join(expected, "\x00") {
		return fmt.Errorf("role inheritance reconciliation mismatch")
	}
	return nil
}

func (p *AuthzCutoverPlan) block(category string) {
	for _, existing := range p.Summary.Blockers {
		if existing == category {
			return
		}
	}
	p.Summary.Blockers = append(p.Summary.Blockers, category)
}

func value(input *string) string {
	if input == nil {
		return ""
	}
	return *input
}

func inheritanceCycle(inheritances []*roleinheritance.Inheritance) bool {
	for index, edge := range inheritances {
		if roleinheritance.WouldCreateCycle(inheritances[:index], edge.RoleID, edge.InheritedRoleID) {
			return true
		}
	}
	return false
}

func planHash(grants []*permissiongrant.Grant, inheritances []*roleinheritance.Inheritance) string {
	parts := make([]string, 0, len(grants)+len(inheritances))
	for _, grant := range grants {
		parts = append(parts, "g:"+grant.GrantKey)
	}
	for _, key := range inheritancePlanKeys(inheritances) {
		parts = append(parts, "i:"+key)
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

func inheritancePlanKeys(inheritances []*roleinheritance.Inheritance) []string {
	keys := make([]string, 0, len(inheritances))
	for _, edge := range inheritances {
		keys = append(keys, fmt.Sprintf("%s:%d:%d", edge.TenantIDString(), edge.RoleID.Uint64(), edge.InheritedRoleID.Uint64()))
	}
	sort.Strings(keys)
	return keys
}
