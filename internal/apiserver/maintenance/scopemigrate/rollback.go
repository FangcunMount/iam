package scopemigrate

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	policydomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	assignmentpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/assignment"
	policypo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/policy"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/rolemodel"
	dbmysql "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func rollbackSnapshot(before, after Snapshot, plan Plan, actor meta.ID, at time.Time) (Snapshot, error) {
	if actor <= 0 || at.IsZero() || after.IAM.PolicyVersion == math.MaxInt64 {
		return Snapshot{}, fmt.Errorf("invalid rollback audit/version")
	}
	expected := after
	expected.IAM.Assignments = append([]assignmentpo.AssignmentPO(nil), after.IAM.Assignments...)
	expected.IAM.PolicyVersion++
	originals := map[string]assignmentpo.AssignmentPO{}
	changed := map[string]bool{}
	for _, a := range before.IAM.Assignments {
		originals[a.ID.String()] = a
	}
	for _, c := range plan.Changes {
		changed[c.AssignmentID] = true
	}
	for i := range expected.IAM.Assignments {
		a := &expected.IAM.Assignments[i]
		old, existed := originals[a.ID.String()]
		if existed && !changed[a.ID.String()] {
			continue
		}
		if a.Version == math.MaxUint32 || at.Before(a.UpdatedAt) {
			return Snapshot{}, fmt.Errorf("rollback audit/version conflict")
		}
		a.Version++
		a.UpdatedAt = at
		a.UpdatedBy = actor
		if existed {
			a.OrgID = old.OrgID
			a.ScopeKind = old.ScopeKind
			a.ScopeStoreIDs = old.ScopeStoreIDs
		} else {
			a.DeletedAt = &at
			a.DeletedBy = actor
			a.ActiveGuard = nil
		}
	}
	return expected, nil
}

// Rollback is compensation under a coordinated maintenance window, not a
// version rewind. Later authorization or QS fact changes block automatic work.
func Rollback(ctx context.Context, iam, qs *gorm.DB, stager event.Stager, id, actorID, fingerprint string, stopped bool) (*Receipt, error) {
	actor, err := positive(actorID)
	if err != nil || !stopped || iam == nil || qs == nil || stager == nil || len(fingerprint) != 64 {
		return nil, fmt.Errorf("reviewed fingerprint, actor, databases, outbox and paused writers required")
	}
	var result *Receipt
	err = dbmysql.NewUnitOfWork(iam).WithinTransaction(ctx, func(txCtx context.Context) error {
		tx, e := dbmysql.RequireTx(txCtx)
		if e != nil {
			return e
		}
		q := tx.Table("authz_policy_versions")
		if tx.Dialector.Name() != "sqlite" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		var row struct{ ID uint64 }
		if e = q.Where("id = ?", 1).Take(&row).Error; e != nil {
			return e
		}
		state, e := rolemodel.LoadState(txCtx, tx)
		if e != nil {
			return e
		}
		business, e := LoadBusinessFacts(txCtx, qs)
		if e != nil {
			return e
		}
		current := Snapshot{IAM: state, QS: business}
		var receipt Receipt
		if e = tx.Where("migration_id = ?", id).Take(&receipt).Error; e != nil {
			return e
		}
		report, e := inspect(current, id, &receipt)
		if e != nil {
			return e
		}
		if receipt.Fingerprint != fingerprint {
			return fmt.Errorf("rollback fingerprint mismatch")
		}
		if report.State == "rolled_back" && current.Hash() == receipt.RollbackHash {
			result = &receipt
			return nil
		}
		if report.State != "applied_unchanged" {
			return fmt.Errorf("rollback conflicts with current facts: %s", report.State)
		}
		var before, after Snapshot
		var plan Plan
		if e = json.Unmarshal([]byte(receipt.BeforeJSON), &before); e != nil {
			return e
		}
		if e = json.Unmarshal([]byte(receipt.AfterJSON), &after); e != nil {
			return e
		}
		if e = json.Unmarshal([]byte(receipt.PlanJSON), &plan); e != nil {
			return e
		}
		now := time.Now().UTC().Truncate(time.Second)
		expected, e := rollbackSnapshot(before, after, plan, actor, now)
		if e != nil {
			return e
		}
		prior := map[string]assignmentpo.AssignmentPO{}
		for _, a := range after.IAM.Assignments {
			prior[a.ID.String()] = a
		}
		for _, a := range expected.IAM.Assignments {
			old := prior[a.ID.String()]
			if a.Version == old.Version {
				continue
			}
			r := tx.Table("authz_assignments").Where("id = ? AND version = ?", a.ID, a.Version-1).Updates(map[string]any{"org_id": a.OrgID, "scope_kind": a.ScopeKind, "scope_store_ids": a.ScopeStoreIDs, "updated_at": a.UpdatedAt, "updated_by": a.UpdatedBy, "deleted_at": a.DeletedAt, "deleted_by": int64(a.DeletedBy), "version": a.Version})
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected != 1 {
				return fmt.Errorf("assignment changed during rollback")
			}
		}
		version, e := policypo.NewPolicyVersionRepository(tx).Increment(txCtx, "user:"+actor.String(), "assignment Scope rollback "+id)
		if e != nil {
			return e
		}
		if e = stager.Stage(txCtx, policydomain.NewVersionChangedEvent(version.Version)); e != nil {
			return e
		}
		actualState, e := rolemodel.LoadState(txCtx, tx)
		if e != nil {
			return e
		}
		actualBusiness, e := LoadBusinessFacts(txCtx, qs)
		if e != nil {
			return e
		}
		actual := Snapshot{IAM: actualState, QS: actualBusiness}
		if actual.Hash() != expected.Hash() {
			return fmt.Errorf("rollback facts differ from reviewed compensation")
		}
		receipt.Status = "rolled_back"
		receipt.RollbackActorID = actor.String()
		receipt.RollbackHash = actual.Hash()
		receipt.RollbackJSON = encode(actual)
		receipt.UpdatedAt = now
		if e = tx.Model(&Receipt{}).Where("migration_id = ? AND status = ?", id, "applied").Updates(map[string]any{"status": receipt.Status, "rollback_actor_id": receipt.RollbackActorID, "rollback_hash": receipt.RollbackHash, "rollback_json": receipt.RollbackJSON, "updated_at": now}).Error; e != nil {
			return e
		}
		result = &receipt
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
