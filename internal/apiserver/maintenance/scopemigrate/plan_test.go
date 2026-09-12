package scopemigrate

import (
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"
	assignmentpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/assignment"
	rolepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/role"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/rolemodel"
	"strings"
	"testing"
)

func fixture() (rolemodel.State, BusinessFacts, Input) {
	r := rolepo.RolePO{Name: "qs:assessment_operator"}
	r.ID = 2
	a := assignmentpo.AssignmentPO{SubjectType: "user", SubjectID: "3", RoleID: 2}
	a.ID = 1
	return rolemodel.State{PolicyVersion: 5, Roles: []rolepo.RolePO{r}, Assignments: []assignmentpo.AssignmentPO{a}}, BusinessFacts{Operators: []OperatorFact{{ID: "4", OrgID: "7", UserID: "3", Active: true}}, Stores: []StoreFact{{ID: "8", OrgID: "7", Active: true}}}, Input{Version: 1, Mappings: []Mapping{{AssignmentID: "1", SubjectID: "3", RoleID: "2", Targets: []Target{{OrgID: "7", Kind: scope.Stores, StoreIDs: []string{"8"}}}}}}
}
func TestExplicitMappingAndNoImplicitCompanyGrant(t *testing.T) {
	state, qs, input := fixture()
	p := Build(state, qs, input)
	if err := p.Validate(); err != nil || len(p.Changes) != 1 || p.Changes[0].Targets[0].Kind != scope.Stores {
		t.Fatalf("plan %+v: %v", p, err)
	}
	input.Mappings = nil
	if Build(state, qs, input).Validate() == nil {
		t.Fatal("missing mapping inferred access")
	}
}
func TestBootstrapRejectsConflictingFacts(t *testing.T) {
	for _, kind := range []string{"inactive", "foreign store", "duplicate membership", "role mismatch", "unknown role", "already scoped", "empty targets", "duplicate mapping", "duplicate company", "invalid existing scope"} {
		t.Run(kind, func(t *testing.T) {
			state, qs, input := fixture()
			switch kind {
			case "inactive":
				qs.Operators[0].Active = false
			case "foreign store":
				qs.Stores[0].OrgID = "9"
			case "duplicate membership":
				copy := qs.Operators[0]
				copy.ID = "6"
				qs.Operators = append(qs.Operators, copy)
			case "role mismatch":
				input.Mappings[0].RoleID = "99"
			case "unknown role":
				state.Roles[0].Name = "qs:new_role"
			case "already scoped":
				a := state.Assignments[0]
				a.ID = 9
				a.OrgID = 7
				a.ScopeKind = "all_stores"
				state.Assignments = append(state.Assignments, a)
			case "empty targets":
				input.Mappings[0].Targets = nil
			case "duplicate mapping":
				input.Mappings = append(input.Mappings, input.Mappings[0])
			case "duplicate company":
				input.Mappings[0].Targets = append(input.Mappings[0].Targets, input.Mappings[0].Targets[0])
			case "invalid existing scope":
				state.Assignments[0].OrgID = 7
				state.Assignments[0].ScopeKind = "unknown"
				input.Mappings = nil
			}
			if Build(state, qs, input).Validate() == nil {
				t.Fatal("unsafe bootstrap accepted")
			}
		})
	}
}
func TestExplicitMultipleCompanyTargets(t *testing.T) {
	state, qs, input := fixture()
	qs.Operators = append(qs.Operators, OperatorFact{ID: "5", OrgID: "9", UserID: "3", Active: true})
	input.Mappings[0].Targets = append(input.Mappings[0].Targets, Target{OrgID: "9", Kind: scope.AllStores})
	p := Build(state, qs, input)
	if p.Validate() != nil || len(p.Changes[0].Targets) != 2 {
		t.Fatalf("multi-company explicit assignment %+v", p)
	}
	before := p.Fingerprint
	qs.Operators[0].Active = false
	if Build(state, qs, input).Fingerprint == before {
		t.Fatal("operator status omitted from fingerprint")
	}
}
func TestMappingContractRejectsUnknownAndTrailingInput(t *testing.T) {
	for _, raw := range []string{`{"version":2}`, `{"version":1,"company_wide":true}`, `{"version":1} {}`} {
		if _, err := Decode(strings.NewReader(raw)); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
}
