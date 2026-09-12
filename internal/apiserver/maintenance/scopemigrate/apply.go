package scopemigrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	assignmentdomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/assignment"
	policydomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"
	assignmentpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/assignment"
	policypo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/policy"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/rolemodel"
	dbmysql "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func encode(v any) string { b, _ := json.Marshal(v); return string(b) }

// Apply requires an actual maintenance window across IAM authorization and QS
// membership/store writers. It never converts missing mappings to all stores.
func Apply(ctx context.Context, iam, qs *gorm.DB, stager event.Stager, id, actorID, fingerprint string, input Input, stopped bool) (*Receipt, error) {
	actor, err := positive(actorID)
	if err != nil || !stopped || len(fingerprint) != 64 || iam == nil || qs == nil || stager == nil {
		return nil, fmt.Errorf("reviewed fingerprint, actor, databases, durable outbox and paused writers required")
	}
	if id == "" || len(id) > 64 || strings.TrimSpace(id) != id {
		return nil, fmt.Errorf("valid migration ID required")
	}
	if !iam.Migrator().HasTable(&Receipt{}) {
		return nil, fmt.Errorf("install scope receipt schema first")
	}
	var result *Receipt
	err = dbmysql.NewUnitOfWork(iam).WithinTransaction(ctx, func(txCtx context.Context) error {
		tx, e := dbmysql.RequireTx(txCtx)
		if e != nil {
			return e
		}
		lock := tx.Table("authz_policy_versions")
		if tx.Dialector.Name() != "sqlite" {
			lock = lock.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		var policy struct{ ID uint64 }
		if e = lock.Where("id = ?", 1).Take(&policy).Error; e != nil {
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
		before := Snapshot{IAM: state, QS: business}
		var receipt Receipt
		e = tx.Where("migration_id = ?", id).Take(&receipt).Error
		if e == nil {
			report, checkErr := inspect(before, id, &receipt)
			if checkErr != nil {
				return checkErr
			}
			if report.State != "applied_unchanged" || receipt.Fingerprint != fingerprint {
				return fmt.Errorf("scope migration conflicts: %s", report.State)
			}
			result = &receipt
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		plan := Build(state, business, input)
		if e = plan.Validate(); e != nil {
			return e
		}
		if plan.Fingerprint != fingerprint {
			return fmt.Errorf("reviewed facts changed")
		}
		now := time.Now().UTC().Truncate(time.Second)
		if e = applyAssignments(tx, before, plan, actor, now); e != nil {
			return e
		}
		version, e := policypo.NewPolicyVersionRepository(tx).Increment(txCtx, "user:"+actor.String(), "assignment Scope bootstrap "+id)
		if e != nil {
			return e
		}
		if e = stager.Stage(txCtx, policydomain.NewVersionChangedEvent(version.Version)); e != nil {
			return e
		}
		afterState, e := rolemodel.LoadState(txCtx, tx)
		if e != nil {
			return e
		}
		afterBusiness, e := LoadBusinessFacts(txCtx, qs)
		if e != nil {
			return e
		}
		after := Snapshot{IAM: afterState, QS: afterBusiness}
		if e = ValidateTransition(before, after, plan, actor); e != nil {
			return e
		}
		result = &Receipt{MigrationID: id, ActorID: actor.String(), Fingerprint: fingerprint, AfterHash: after.Hash(), Status: "applied", BeforeJSON: encode(before), AfterJSON: encode(after), InputJSON: encode(input), PlanJSON: encode(plan), CreatedAt: now, UpdatedAt: now}
		return tx.Create(result).Error
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
func applyAssignments(tx *gorm.DB, before Snapshot, p Plan, actor meta.ID, now time.Time) error {
	rows := map[string]assignmentpo.AssignmentPO{}
	for _, a := range before.IAM.Assignments {
		rows[a.ID.String()] = a
	}
	for _, c := range p.Changes {
		old, ok := rows[c.AssignmentID]
		if !ok {
			return fmt.Errorf("missing assignment")
		}
		for i, target := range c.Targets {
			org, err := positive(target.OrgID)
			if err != nil {
				return err
			}
			ids := make([]meta.ID, 0, len(target.StoreIDs))
			for _, raw := range target.StoreIDs {
				value, e := positive(raw)
				if e != nil {
					return e
				}
				ids = append(ids, value)
			}
			value, err := scope.New(org, target.Kind, ids)
			if err != nil {
				return err
			}
			if i == 0 {
				result := tx.Table("authz_assignments").Where("id = ? AND version = ? AND deleted_at IS NULL AND org_id = 0 AND scope_kind = '' AND scope_store_ids IS NULL", old.ID, old.Version).Updates(map[string]any{"org_id": org, "scope_kind": string(value.Kind()), "scope_store_ids": encode(target.StoreIDs), "updated_at": now, "updated_by": actor, "version": gorm.Expr("version + 1")})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return fmt.Errorf("assignment changed during bootstrap")
				}
				continue
			}
			user, err := positive(c.SubjectID)
			if err != nil {
				return err
			}
			role, err := positive(c.RoleID)
			if err != nil {
				return err
			}
			bo, err := assignmentdomain.NewAssignment(assignmentdomain.SubjectTypeUser, user, role, assignmentdomain.WithGrantedBy("user:"+actor.String()), assignmentdomain.WithScope(value))
			if err != nil {
				return err
			}
			row := assignmentpo.NewMapper().ToPO(&bo)
			row.ID = meta.New()
			row.Version = 1
			row.CreatedAt = now
			row.UpdatedAt = now
			row.GrantedAt = now
			row.CreatedBy = actor
			row.UpdatedBy = actor
			// Explicit audit fields and ID are part of the migration manifest; do not let generic hooks replace them.
			if err = tx.Session(&gorm.Session{SkipHooks: true}).Create(row).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
