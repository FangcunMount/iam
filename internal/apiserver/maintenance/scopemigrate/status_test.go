package scopemigrate

import (
	"encoding/json"
	"testing"
	"time"
)

func encoded(v any) string { b, _ := json.Marshal(v); return string(b) }
func receiptFixture() (Snapshot, *Receipt) {
	state, qs, input := fixture()
	before := Snapshot{IAM: state, QS: qs}
	// A durable receipt binds both fact snapshots and the reviewed mapping.
	beforeJSON := encoded(before)
	plan := Build(state, qs, input)
	var after Snapshot
	if err := json.Unmarshal([]byte(beforeJSON), &after); err != nil {
		panic(err)
	}
	after.IAM.PolicyVersion++
	after.IAM.Assignments[0].Version++
	after.IAM.Assignments[0].UpdatedBy = 9
	after.IAM.Assignments[0].UpdatedAt = time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	after.IAM.Assignments[0].OrgID = 7
	after.IAM.Assignments[0].ScopeKind = "stores"
	ids := `["8"]`
	after.IAM.Assignments[0].ScopeStoreIDs = &ids
	return after, &Receipt{ActorID: "9", MigrationID: "scope-v1", Status: "applied", Fingerprint: before.Hash(), AfterHash: after.Hash(), BeforeJSON: beforeJSON, AfterJSON: encoded(after), InputJSON: encoded(input), PlanJSON: encoded(plan)}
}
func TestScopeStatusHistoricalCompletionAndDrift(t *testing.T) {
	for _, kind := range []string{"unchanged", "iam drift", "qs drift", "rolled back", "corrupt input", "corrupt archive", "corrupt plan", "unknown status", "not applied"} {
		t.Run(kind, func(t *testing.T) {
			current, receipt := receiptFixture()
			want := "applied_unchanged"
			invalid := false
			switch kind {
			case "iam drift":
				current.IAM.PolicyVersion++
				want = "applied_drifted"
			case "qs drift":
				current.QS.Operators[0].Active = false
				want = "applied_drifted"
			case "rolled back":
				receipt.Status = "rolled_back"
				var before Snapshot
				var plan Plan
				if err := json.Unmarshal([]byte(receipt.BeforeJSON), &before); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(receipt.PlanJSON), &plan); err != nil {
					t.Fatal(err)
				}
				receipt.UpdatedAt = time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
				rolled, e := rollbackSnapshot(before, current, plan, 9, receipt.UpdatedAt)
				if e != nil {
					t.Fatal(e)
				}
				receipt.RollbackActorID = "9"
				receipt.RollbackJSON = encoded(rolled)
				receipt.RollbackHash = rolled.Hash()
				current = rolled
				want = "rolled_back"
			case "corrupt input":
				receipt.InputJSON = `{}`
				invalid = true
			case "corrupt archive":
				receipt.BeforeJSON = `{}`
				invalid = true
			case "corrupt plan":
				receipt.PlanJSON = `{}`
				invalid = true
			case "unknown status":
				receipt.Status = "finished"
				invalid = true
			case "not applied":
				receipt.AfterJSON = receipt.BeforeJSON
				receipt.AfterHash = receipt.Fingerprint
				invalid = true
			}
			report, err := inspect(current, "scope-v1", receipt)
			if invalid {
				if err == nil || report.State != "invalid" {
					t.Fatalf("invalid receipt accepted %+v %v", report, err)
				}
				return
			}
			if err != nil || report.State != want {
				t.Fatalf("status %+v %v", report, err)
			}
			if want == "applied_drifted" && len(report.Differences) == 0 {
				t.Fatal("drift without explanation")
			}
		})
	}
}
func TestScopeStatusPendingAndFingerprintCoversFacts(t *testing.T) {
	state, qs, _ := fixture()
	current := Snapshot{IAM: state, QS: qs}
	report, err := inspect(current, "scope-v1", nil)
	if err != nil || report.State != "pending" || report.CurrentFingerprint != current.Hash() {
		t.Fatalf("pending %+v %v", report, err)
	}
	before := current.Hash()
	current.IAM.Assignments[0].GrantedBy = "different"
	if current.Hash() == before {
		t.Fatal("assignment audit change omitted from fingerprint")
	}
}
