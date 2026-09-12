package scopemigrate

import (
	"context"
	assignmentpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/assignment"
	"testing"
)

func TestScopeRollbackRestoresLegacyAndKeepsNewAudit(t *testing.T) {
	iam, qs, input := applyFixture(t)
	ctx := context.Background()
	if err := qs.Exec("INSERT INTO operators VALUES(5,9,3,TRUE,NULL)").Error; err != nil {
		t.Fatal(err)
	}
	input.Mappings[0].Targets = append(input.Mappings[0].Targets, Target{OrgID: "9", Kind: "all_stores", StoreIDs: []string{}})
	p, err := Preflight(ctx, iam, qs, "multi", input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(ctx, iam, qs, transactionalStager{}, "multi", "9", p.Fingerprint, input, true); err != nil {
		t.Fatal(err)
	}
	receipt, err := Rollback(ctx, iam, qs, transactionalStager{}, "multi", "9", p.Fingerprint, true)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Status(ctx, iam, qs, "multi")
	if err != nil || report.State != "rolled_back" {
		t.Fatalf("rollback status %+v %v", report, err)
	}
	current, err := LoadSnapshot(ctx, iam, qs)
	if err != nil || current.IAM.PolicyVersion != 7 || current.Hash() != receipt.RollbackHash {
		t.Fatalf("rollback fingerprint %v", err)
	}
	var rows []assignmentpo.AssignmentPO
	if err := iam.Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != 1 || rows[0].OrgID != 0 || rows[0].ScopeKind != "" || rows[0].ScopeStoreIDs != nil || rows[0].Version != 3 || rows[1].DeletedAt == nil || rows[1].Version != 2 {
		t.Fatalf("invalid compensation %+v", rows)
	}
	if _, err := Rollback(ctx, iam, qs, transactionalStager{}, "multi", "9", p.Fingerprint, true); err != nil {
		t.Fatal(err)
	}
	var n int64
	iam.Table("test_outbox").Count(&n)
	if n != 2 {
		t.Fatalf("repeated rollback emitted event %d", n)
	}
	if _, err := Apply(ctx, iam, qs, transactionalStager{}, "multi", "9", p.Fingerprint, input, true); err == nil {
		t.Fatal("rolled-back ID reapplied")
	}
	if err := qs.Exec("UPDATE operators SET is_active=FALSE WHERE id=5").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := Rollback(ctx, iam, qs, transactionalStager{}, "multi", "9", p.Fingerprint, true); err == nil {
		t.Fatal("post-rollback drift ignored")
	}
}
func TestScopeRollbackFailureAndDriftPreserveFacts(t *testing.T) {
	for _, kind := range []string{"outbox failure", "drift", "wrong fingerprint"} {
		t.Run(kind, func(t *testing.T) {
			iam, qs, input := applyFixture(t)
			ctx := context.Background()
			p, err := Preflight(ctx, iam, qs, "one", input)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Apply(ctx, iam, qs, transactionalStager{}, "one", "9", p.Fingerprint, input, true); err != nil {
				t.Fatal(err)
			}
			if kind == "drift" {
				if err := qs.Exec("UPDATE actor_stores SET is_active=FALSE").Error; err != nil {
					t.Fatal(err)
				}
			}
			fingerprint := p.Fingerprint
			if kind == "wrong fingerprint" {
				fingerprint = "0000000000000000000000000000000000000000000000000000000000000000"
			}
			before, err := LoadSnapshot(ctx, iam, qs)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Rollback(ctx, iam, qs, transactionalStager{fail: kind == "outbox failure"}, "one", "9", fingerprint, true); err == nil {
				t.Fatal("unsafe rollback accepted")
			}
			after, err := LoadSnapshot(ctx, iam, qs)
			if err != nil || after.Hash() != before.Hash() {
				t.Fatal("failed rollback altered facts")
			}
			var receipt Receipt
			iam.First(&receipt, "migration_id = ?", "one")
			if receipt.Status != "applied" || receipt.RollbackHash != "" {
				t.Fatal("failed rollback changed receipt")
			}
			var n int64
			iam.Table("test_outbox").Count(&n)
			if n != 1 {
				t.Fatal("failed rollback retained outbox")
			}
		})
	}
}

// Production migrations require deleted_by=0 for an active assignment.
func TestScopeRollbackPreservesNonNullDeletedByMySQL(t *testing.T) {
	iam, qs, input := applyFixture(t)
	if iam.Dialector.Name() != "mysql" {
		t.Skip("requires isolated MySQL")
	}
	for _, query := range []string{
		"UPDATE authz_assignments SET deleted_by=0 WHERE deleted_by IS NULL",
		"ALTER TABLE authz_assignments MODIFY deleted_by BIGINT NOT NULL DEFAULT 0",
	} {
		if err := iam.Exec(query).Error; err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	plan, err := Preflight(ctx, iam, qs, "audit-column", input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(ctx, iam, qs, transactionalStager{}, "audit-column", "9", plan.Fingerprint, input, true); err != nil {
		t.Fatal(err)
	}
	if _, err := Rollback(ctx, iam, qs, transactionalStager{}, "audit-column", "9", plan.Fingerprint, true); err != nil {
		t.Fatal(err)
	}
	var row assignmentpo.AssignmentPO
	if err := iam.First(&row, 1).Error; err != nil {
		t.Fatal(err)
	}
	if row.DeletedBy != 0 || row.DeletedAt != nil || row.OrgID != 0 || row.ScopeKind != "" {
		t.Fatalf("legacy active assignment not restored: %+v", row)
	}
	state, err := LoadSnapshot(ctx, iam, qs)
	if err != nil || state.IAM.PolicyVersion != 7 {
		t.Fatalf("policy must advance on rollback: %v", err)
	}
	var count int64
	if err := iam.Table("test_outbox").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected apply and rollback events, got %d", count)
	}
}
