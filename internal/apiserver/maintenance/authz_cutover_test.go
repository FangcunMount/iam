package maintenance

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	rolerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

func TestAuthzCutoverAnalysisFailureCategoryHidesDependencyError(t *testing.T) {
	cause := errors.New("sensitive dependency detail")
	err := authzCutoverAnalysisFailure("load_resources_failed", cause)

	require.Equal(t, "load_resources_failed", AuthzCutoverAnalysisFailureCategory(err))
	require.ErrorIs(t, err, cause)
	require.NotContains(t, err.Error(), cause.Error())
	require.Equal(t, "unknown_analysis_failure", AuthzCutoverAnalysisFailureCategory(cause))
}

func TestSplitLegacyActionsSupportsOnlyExactAlternativesAndExplicitWildcard(t *testing.T) {
	actions, blocker := splitLegacyActions("read|list|read")
	require.Empty(t, blocker)
	require.Equal(t, []string{"read", "list"}, actions)

	actions, blocker = splitLegacyActions(".*")
	require.Empty(t, blocker)
	require.Equal(t, []string{"*"}, actions)

	for _, unsupported := range []string{"read.*", "read|", "(read|list)", "read+"} {
		_, blocker = splitLegacyActions(unsupported)
		require.Equal(t, "unsupported_action_expression", blocker, unsupported)
	}
}

func TestConvertLegacyScopeIsFailClosed(t *testing.T) {
	for _, unconditional := range []string{"", "all:*", "  all:*  "} {
		set, blocker := convertLegacyScope(unconditional)
		require.Empty(t, blocker)
		require.True(t, set.IsUnconditional())
	}

	set, blocker := convertLegacyScope("origin:adhoc")
	require.Empty(t, blocker)
	require.False(t, set.IsUnconditional())
	require.NoError(t, set.ValidateAgainst(attribute.AssessmentSchema()))

	for _, invalid := range []string{"all", "origin:", "owner:1", "Origin:adhoc"} {
		_, blocker = convertLegacyScope(invalid)
		require.Equal(t, "invalid_scope", blocker, invalid)
	}
}

func TestConvertLegacyPolicySplitsActionsAndOverridesEvaluatorRetry(t *testing.T) {
	role := &rolerepo.RolePO{Name: "qs:evaluator", TenantID: "tenant-a"}
	role.ID = meta.FromUint64(11)
	catalog, err := resource.NewResource(
		assessmentKey,
		[]string{"read", "retry", "batch_evaluate"},
		resource.WithID(resource.NewResourceID(21)),
		resource.WithAttributeSchema(attribute.AssessmentSchema()),
	)
	require.NoError(t, err)

	rule := legacyRule{
		PType: "p",
		V0:    pointer("role:qs:evaluator"),
		V1:    pointer("tenant-a"),
		V2:    pointer(assessmentKey),
		V3:    pointer("read|retry|batch_evaluate"),
		V4:    pointer("all:*"),
	}
	grants, blocker := convertLegacyPolicy(
		rule,
		map[string]*rolerepo.RolePO{"tenant-a\x00qs:evaluator": role},
		map[string]*resource.Resource{assessmentKey: &catalog},
	)
	require.Empty(t, blocker)
	require.Len(t, grants, 3)
	for _, grant := range grants {
		if grant.ActionString() == "retry" {
			require.True(t, grant.IsConditional())
			require.NoError(t, grant.Constraints.ValidateAgainst(attribute.AssessmentSchema()))
			continue
		}
		require.False(t, grant.IsConditional(), grant.ActionString())
	}
}

func TestConvertLegacyPolicyRejectsUnregisteredOriginValue(t *testing.T) {
	role := &rolerepo.RolePO{Name: "role-a", TenantID: "tenant-a"}
	role.ID = meta.FromUint64(11)
	catalog, err := resource.NewResource(
		assessmentKey,
		[]string{"retry"},
		resource.WithID(resource.NewResourceID(21)),
		resource.WithAttributeSchema(attribute.AssessmentSchema()),
	)
	require.NoError(t, err)
	rule := legacyRule{
		PType: "p", V0: pointer("role:role-a"), V1: pointer("tenant-a"),
		V2: pointer(assessmentKey), V3: pointer("retry"), V4: pointer("origin:unknown"),
	}
	_, blocker := convertLegacyPolicy(
		rule,
		map[string]*rolerepo.RolePO{"tenant-a\x00role-a": role},
		map[string]*resource.Resource{assessmentKey: &catalog},
	)
	require.Equal(t, "scope_attribute_schema_mismatch", blocker)
}

func TestPersistCutoverVerificationIsIdempotentForSameEvidence(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE authz_cutover_state (
		id INTEGER PRIMARY KEY,
		status TEXT NOT NULL,
		evidence_hash TEXT NOT NULL,
		verified_at DATETIME NULL
	)`).Error)

	first := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	require.NoError(t, persistCutoverVerification(db, "hash-a", first))
	require.NoError(t, persistCutoverVerification(db, "hash-a", first.Add(time.Hour)))

	var unchanged cutoverVerificationState
	require.NoError(t, db.Table("authz_cutover_state").Where("id = ?", 1).Take(&unchanged).Error)
	require.Equal(t, "hash-a", unchanged.EvidenceHash)
	require.NotNil(t, unchanged.VerifiedAt)
	require.True(t, unchanged.VerifiedAt.Equal(first))

	second := first.Add(2 * time.Hour)
	require.NoError(t, persistCutoverVerification(db, "hash-b", second))
	var updated cutoverVerificationState
	require.NoError(t, db.Table("authz_cutover_state").Where("id = ?", 1).Take(&updated).Error)
	require.Equal(t, "hash-b", updated.EvidenceHash)
	require.NotNil(t, updated.VerifiedAt)
	require.True(t, updated.VerifiedAt.Equal(second))
}

func TestPilotAssessmentRetryGrantsAlwaysMaterializeFinalMatrix(t *testing.T) {
	roles := make(map[string]*rolerepo.RolePO)
	for id, name := range map[uint64]string{
		11: "qs:admin", 12: "qs:evaluator", 13: "qs:evaluation_plan_manager",
	} {
		role := &rolerepo.RolePO{Name: name, TenantID: "fangcun"}
		role.ID = meta.FromUint64(id)
		roles["fangcun\x00"+name] = role
	}
	catalog, err := resource.NewResource(
		assessmentKey,
		[]string{"retry", "force_retry", "batch_evaluate"},
		resource.WithID(resource.NewResourceID(21)),
		resource.WithAttributeSchema(attribute.AssessmentSchema()),
	)
	require.NoError(t, err)
	plan := &AuthzCutoverPlan{}

	addPilotAssessmentRetryGrants(
		plan,
		roles,
		map[string]*resource.Resource{assessmentKey: &catalog},
		map[string]struct{}{},
	)

	require.Empty(t, plan.Summary.Blockers)
	require.Len(t, plan.grants, 3)
	for _, grant := range plan.grants {
		switch grant.RoleID.Uint64() {
		case 11:
			require.Equal(t, "qs:*:*:*", grant.ResourcePatternString())
			require.Equal(t, "*", grant.ActionString())
			require.True(t, grant.Constraints.IsUnconditional())
		case 12:
			require.Equal(t, "retry", grant.ActionString())
			evaluation, evalErr := grant.Constraints.Evaluate(map[string]constraint.Value{
				attribute.ObjectOriginType: constraint.StringValue("adhoc"),
			})
			require.NoError(t, evalErr)
			require.True(t, evaluation.Matched)
		case 13:
			evaluation, evalErr := grant.Constraints.Evaluate(map[string]constraint.Value{
				attribute.ObjectOriginType: constraint.StringValue("plan"),
			})
			require.NoError(t, evalErr)
			require.True(t, evaluation.Matched)
		default:
			t.Fatalf("unexpected pilot role id %d", grant.RoleID.Uint64())
		}
	}
}

func pointer(value string) *string { return &value }
