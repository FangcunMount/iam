package accountprovision

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"reflect"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authn/authentication"
	policydomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	assignmentpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/assignment"
	policypo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/policy"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/rolemodel"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/scopemigrate"
	dbmysql "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReviewerScopeReport is private evidence. It contains no credentials. The full
// authorization snapshot makes source/grant drift visible and preserves before facts.
type ReviewerScopeReport struct {
	State       string            `json:"state"`
	Fingerprint string            `json:"fingerprint"`
	OrgID       string            `json:"org_id"`
	Plan        scopemigrate.Plan `json:"plan"`
	Before      rolemodel.State   `json:"before"`
	After       *rolemodel.State  `json:"after,omitempty"`
}

// reviewerScopePlan is deliberately limited to the existing reviewer and the
// same three direct roles. It cannot create roles, overwrite configured ranges,
// infer all_stores, or run a global legacy assignment migration.
func reviewerScopePlan(state rolemodel.State, org meta.ID) (scopemigrate.Plan, error) {
	p := scopemigrate.Plan{Fingerprint: (scopemigrate.Snapshot{IAM: state}).Hash(), Changes: []scopemigrate.Change{}}
	names := map[uint64]string{}
	for _, r := range state.Roles {
		if r.DeletedAt == nil {
			names[r.ID.Uint64()] = r.Name
		}
	}
	source, target := map[uint64]assignmentpo.AssignmentPO{}, map[uint64]assignmentpo.AssignmentPO{}
	for _, a := range state.Assignments {
		if a.DeletedAt != nil || a.SubjectType != "user" || (a.SubjectID != "10001" && a.SubjectID != "10002") {
			continue
		}
		name := names[a.RoleID]
		if name != "platform_admin" && name != "iam_admin" && name != "qs:admin" {
			return p, fmt.Errorf("unexpected reviewer/reference role")
		}
		rows := source
		if a.SubjectID == "10002" {
			rows = target
		}
		if _, ok := rows[a.RoleID]; ok {
			return p, fmt.Errorf("ambiguous multi-company role assignment")
		}
		rows[a.RoleID] = a
	}
	if org <= 0 || state.PolicyVersion <= 0 || state.PolicyVersion == math.MaxInt64 || len(source) != 3 || len(target) != 3 {
		return p, fmt.Errorf("both existing administrators must have exactly the reviewed roles")
	}
	for _, a := range state.Assignments {
		if a.DeletedAt != nil || a.SubjectType != "user" || a.SubjectID != "10001" {
			continue
		}
		b, ok := target[a.RoleID]
		if !ok {
			return p, fmt.Errorf("reference and reviewer roles differ")
		}
		name := names[a.RoleID]
		if name == "iam_admin" {
			if a.OrgID != b.OrgID || a.ScopeKind != b.ScopeKind || !reflect.DeepEqual(a.ScopeStoreIDs, b.ScopeStoreIDs) {
				return p, fmt.Errorf("IAM-only role differs; requires separate review")
			}
			continue
		}
		// Reuse strict scope mapping validation; raw source bytes are used for the write.
		if a.OrgID != org.Uint64() || a.ScopeStoreIDs == nil {
			return p, fmt.Errorf("reference company scope is missing or differs from QS membership")
		}
		bo, err := assignmentpo.NewMapper().ToBO(&a)
		if err != nil {
			return p, err
		}
		value, configured := bo.Scope()
		if !configured {
			return p, fmt.Errorf("reference scope unconfigured")
		}
		t := scopemigrate.Target{OrgID: org.String(), Kind: value.Kind(), StoreIDs: []string{}}
		for _, id := range value.StoreIDs() {
			t.StoreIDs = append(t.StoreIDs, id.String())
		}
		if a.OrgID == b.OrgID && a.ScopeKind == b.ScopeKind && reflect.DeepEqual(a.ScopeStoreIDs, b.ScopeStoreIDs) {
			continue
		}
		if b.OrgID != 0 || b.ScopeKind != "" || b.ScopeStoreIDs != nil || b.Version == math.MaxUint32 {
			return p, fmt.Errorf("reviewer already configured differently; refusing overwrite")
		}
		p.Changes = append(p.Changes, scopemigrate.Change{AssignmentID: b.ID.String(), SubjectID: "10002", RoleID: fmt.Sprint(b.RoleID), RoleName: name, BeforeScope: "unconfigured", Targets: []scopemigrate.Target{t}})
	}
	return p, nil
}

func reviewerScopeInspect(ctx context.Context, db *gorm.DB, input Input, hasher authentication.PasswordHasher, org meta.ID) (ReviewerScopeReport, error) {
	var r ReviewerScopeReport
	if input.UserID != "10002" || input.ActorID != "10001" || input.Username != "review@mfangcunmount.com" || input.RequestID != "reviewer-10002-actions" {
		return r, fmt.Errorf("only the existing fixed reviewer can be repaired")
	}
	account, err := Preflight(ctx, db, input, hasher)
	if err != nil {
		return r, err
	}
	if account.State != "historical_completed" {
		return r, fmt.Errorf("account must already exist under the original provisioning request")
	}
	r.Before, err = rolemodel.LoadState(ctx, db)
	if err != nil {
		return r, err
	}
	r.Plan, err = reviewerScopePlan(r.Before, org)
	if err != nil {
		return r, err
	}
	r.OrgID = org.String()
	r.Fingerprint = r.Plan.Fingerprint
	r.State = "needs_scope_repair"
	if len(r.Plan.Changes) == 0 {
		r.State = "scope_matches_reference"
	}
	return r, nil
}

func ReviewerScopePreflight(ctx context.Context, db *gorm.DB, input Input, hasher authentication.PasswordHasher, org meta.ID) (ReviewerScopeReport, error) {
	var r ReviewerScopeReport
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var e error
		r, e = reviewerScopeInspect(ctx, tx, input, hasher, org)
		return e
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return r, err
}

func ReviewerScopeApply(ctx context.Context, db *gorm.DB, input Input, hasher authentication.PasswordHasher, org meta.ID, fingerprint string, stager event.Stager) (ReviewerScopeReport, error) {
	var r ReviewerScopeReport
	if len(fingerprint) != 64 || stager == nil {
		return r, fmt.Errorf("reviewed fingerprint and durable outbox required")
	}
	err := dbmysql.NewUnitOfWork(db).WithinTransaction(ctx, func(txctx context.Context) error {
		tx, e := dbmysql.RequireTx(txctx)
		if e != nil {
			return e
		}
		lock := tx.Table("authz_policy_versions")
		if tx.Dialector.Name() != "sqlite" {
			lock = lock.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		var version struct{ PolicyVersion int64 }
		if e = lock.Where("id=1").Take(&version).Error; e != nil {
			return e
		}
		r, e = reviewerScopeInspect(txctx, tx, input, hasher, org)
		if e != nil {
			return e
		}
		if r.Fingerprint != fingerprint {
			return fmt.Errorf("authorization facts changed; repeat preflight")
		}
		if len(r.Plan.Changes) == 0 {
			return nil
		}
		before := scopemigrate.Snapshot{IAM: r.Before}
		now := time.Now().UTC().Truncate(time.Second)
		for _, c := range r.Plan.Changes {
			var old, source assignmentpo.AssignmentPO
			for _, a := range r.Before.Assignments {
				if a.ID.String() == c.AssignmentID {
					old = a
				}
				if a.DeletedAt == nil && a.SubjectType == "user" && a.SubjectID == "10001" && fmt.Sprint(a.RoleID) == c.RoleID {
					source = a
				}
			}
			update := tx.Table("authz_assignments").Where("id=? AND subject_type='user' AND subject_id='10002' AND version=? AND deleted_at IS NULL AND org_id=0 AND scope_kind='' AND scope_store_ids IS NULL", old.ID, old.Version).Updates(map[string]any{"org_id": source.OrgID, "scope_kind": source.ScopeKind, "scope_store_ids": *source.ScopeStoreIDs, "updated_at": now, "updated_by": input.ActorID, "version": gorm.Expr("version + 1")})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return fmt.Errorf("reviewer scope changed concurrently")
			}
		}
		v, e := policypo.NewPolicyVersionRepository(tx).Increment(txctx, "user:10001", "reviewer-10002 scope repair from reference 10001")
		if e != nil {
			return e
		}
		if e = stager.Stage(txctx, policydomain.NewVersionChangedEvent(v.Version)); e != nil {
			return e
		}
		after, e := rolemodel.LoadState(txctx, tx)
		if e != nil {
			return e
		}
		if e = scopemigrate.ValidateTransition(before, scopemigrate.Snapshot{IAM: after}, r.Plan, meta.ID(10001)); e != nil {
			return e
		}
		p, e := reviewerScopePlan(after, org)
		if e != nil {
			return e
		}
		if len(p.Changes) != 0 {
			return fmt.Errorf("repair incomplete")
		}
		r.After = &after
		r.State = "scope_matches_reference"
		return nil
	})
	return r, err
}
