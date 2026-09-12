package scopemigrate

import (
	"encoding/json"
	"testing"
)

func TestTransitionRejectsUnreviewedChangesEvenWithRecomputedChecksum(t *testing.T) {
	for _, kind := range []string{"wrong store", "extra assignment", "removed assignment", "changed grantor", "changed role", "changed role catalog", "changed company facts", "wrong audit actor", "wrong version"} {
		t.Run(kind, func(t *testing.T) {
			after, receipt := receiptFixture()
			var before Snapshot
			var p Plan
			if err := json.Unmarshal([]byte(receipt.BeforeJSON), &before); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(receipt.PlanJSON), &p); err != nil {
				t.Fatal(err)
			}
			if err := ValidateTransition(before, after, p, 9); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "wrong store":
				ids := `["99"]`
				after.IAM.Assignments[0].ScopeStoreIDs = &ids
			case "extra assignment":
				a := after.IAM.Assignments[0]
				a.ID = 99
				after.IAM.Assignments = append(after.IAM.Assignments, a)
			case "removed assignment":
				after.IAM.Assignments = nil
			case "changed grantor":
				after.IAM.Assignments[0].GrantedBy = "user:99"
			case "changed role":
				after.IAM.Assignments[0].RoleID = 99
			case "changed role catalog":
				after.IAM.Roles[0].Name = "qs:admin"
			case "changed company facts":
				after.QS.Operators[0].Active = false
			case "wrong audit actor":
				after.IAM.Assignments[0].UpdatedBy = 99
			case "wrong version":
				after.IAM.Assignments[0].Version++
			}
			receipt.AfterJSON = encoded(after)
			receipt.AfterHash = after.Hash()
			report, err := inspect(after, "scope-v1", receipt)
			if err == nil || report.State != "invalid" {
				t.Fatalf("tampered transition accepted: %+v %v", report, err)
			}
		})
	}
}
