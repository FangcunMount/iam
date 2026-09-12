package scopemigrate

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/rolemodel"
	"gorm.io/gorm"
	"strings"
	"time"
)

type Receipt struct {
	RollbackActorID string    `json:"rollback_actor_id,omitempty"`
	RollbackHash    string    `json:"rollback_hash,omitempty"`
	RollbackJSON    string    `json:"-"`
	ActorID         string    `json:"actor_id"`
	MigrationID     string    `gorm:"primaryKey;size:64" json:"migration_id"`
	Fingerprint     string    `json:"fingerprint"`
	AfterHash       string    `json:"after_hash"`
	Status          string    `json:"status"`
	BeforeJSON      string    `json:"-"`
	AfterJSON       string    `json:"-"`
	InputJSON       string    `json:"-"`
	PlanJSON        string    `json:"-"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (Receipt) TableName() string { return "iam_scope_migrations" }

type Report struct {
	State              string   `json:"state"`
	NextAction         string   `json:"next_action"`
	MigrationID        string   `json:"migration_id"`
	Fingerprint        string   `json:"fingerprint"`
	AfterHash          string   `json:"after_hash"`
	CurrentFingerprint string   `json:"current_fingerprint"`
	Differences        []string `json:"differences"`
	Plan               *Plan    `json:"plan,omitempty"`
}

func inspect(current Snapshot, id string, receipt *Receipt) (Report, error) {
	report := Report{State: "invalid", NextAction: "stop", MigrationID: id, CurrentFingerprint: current.Hash(), Differences: []string{}}
	if strings.TrimSpace(id) != id || id == "" || len(id) > 64 {
		return report, fmt.Errorf("valid migration ID required")
	}
	if receipt == nil {
		report.State = "pending"
		report.NextAction = "preflight"
		return report, nil
	}
	report.Fingerprint, report.AfterHash = receipt.Fingerprint, receipt.AfterHash
	var before, after Snapshot
	var plan Plan
	input, err := Decode(strings.NewReader(receipt.InputJSON))
	if err != nil || receipt.MigrationID != id || json.Unmarshal([]byte(receipt.BeforeJSON), &before) != nil || json.Unmarshal([]byte(receipt.AfterJSON), &after) != nil || json.Unmarshal([]byte(receipt.PlanJSON), &plan) != nil {
		return report, fmt.Errorf("invalid scope migration archive")
	}
	if before.Hash() != receipt.Fingerprint || after.Hash() != receipt.AfterHash || !EqualPlan(Build(before.IAM, before.QS, input), plan) || plan.Validate() != nil {
		return report, fmt.Errorf("scope migration archive checksum or plan mismatch")
	}
	actor, actorErr := positive(receipt.ActorID)
	if actorErr != nil || ValidateTransition(before, after, plan, actor) != nil {
		return report, fmt.Errorf("invalid archived assignment transition")
	}
	afterPlan := Build(after.IAM, after.QS, Input{Version: 1})
	if after.IAM.PolicyVersion != before.IAM.PolicyVersion+1 || afterPlan.Validate() != nil || len(afterPlan.Changes) != 0 {
		return report, fmt.Errorf("archive does not contain a completed scope transition")
	}
	report.Plan = &plan
	switch receipt.Status {
	case "rolled_back":
		var rolled Snapshot
		rollbackActor, e := positive(receipt.RollbackActorID)
		if e != nil || json.Unmarshal([]byte(receipt.RollbackJSON), &rolled) != nil || rolled.Hash() != receipt.RollbackHash {
			return report, fmt.Errorf("invalid rollback archive")
		}
		expected, e := rollbackSnapshot(before, after, plan, rollbackActor, receipt.UpdatedAt)
		if e != nil || expected.Hash() != rolled.Hash() {
			return report, fmt.Errorf("invalid rollback transition")
		}
		report.State = "rolled_back"
		report.NextAction = "review_rollback"
	case "applied":
		if report.CurrentFingerprint == receipt.AfterHash {
			report.State = "applied_unchanged"
			report.NextAction = "verify"
		} else {
			report.State = "applied_drifted"
			report.NextAction = "review_differences"
		}
	default:
		return report, fmt.Errorf("unknown scope migration status")
	}
	if current.IAM.Hash() != after.IAM.Hash() {
		report.Differences = append(report.Differences, "IAM authorization facts or policy version changed")
	}
	if (Snapshot{QS: current.QS}).Hash() != (Snapshot{QS: after.QS}).Hash() {
		report.Differences = append(report.Differences, "QS Operator or store facts changed")
	}
	return report, nil
}

// Status reads IAM facts and the receipt in one repeatable-read snapshot, so a
// concurrent migration commit cannot pair old assignments with a new receipt.
func Status(ctx context.Context, iam, qs *gorm.DB, id string) (Report, error) {
	report := Report{State: "invalid", NextAction: "stop", MigrationID: id}
	if iam == nil || qs == nil {
		return report, fmt.Errorf("IAM and QS databases required")
	}
	err := iam.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state, e := rolemodel.LoadState(ctx, tx)
		if e != nil {
			return e
		}
		business, e := LoadBusinessFacts(ctx, qs)
		if e != nil {
			return e
		}
		current := Snapshot{IAM: state, QS: business}
		var receipt Receipt
		var saved *Receipt
		if tx.Migrator().HasTable(&Receipt{}) {
			e = tx.Where("migration_id = ?", id).Take(&receipt).Error
			if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
				return e
			}
			if e == nil {
				saved = &receipt
			}
		}
		report, e = inspect(current, id, saved)
		return e
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return report, err
}
func Preflight(ctx context.Context, iam, qs *gorm.DB, id string, input Input) (Report, error) {
	report, err := Status(ctx, iam, qs, id)
	if err != nil {
		return report, err
	}
	if report.State == "applied_unchanged" {
		return report, nil
	}
	if report.State != "pending" {
		return report, fmt.Errorf("scope migration not executable: %s", report.State)
	}
	current, err := LoadSnapshot(ctx, iam, qs)
	if err != nil {
		return report, err
	}
	if current.Hash() != report.CurrentFingerprint {
		return report, fmt.Errorf("facts changed during preflight")
	}
	plan := Build(current.IAM, current.QS, input)
	report.Plan = &plan
	report.Fingerprint = plan.Fingerprint
	if err := plan.Validate(); err != nil {
		report.NextAction = "resolve_issues"
		return report, err
	}
	report.NextAction = "review_and_apply"
	return report, nil
}
func Verify(ctx context.Context, iam, qs *gorm.DB, id string) (Report, error) {
	report, err := Status(ctx, iam, qs, id)
	if err != nil {
		return report, err
	}
	if report.State != "applied_unchanged" {
		return report, fmt.Errorf("scope migration verification failed: %s", report.State)
	}
	return report, nil
}
