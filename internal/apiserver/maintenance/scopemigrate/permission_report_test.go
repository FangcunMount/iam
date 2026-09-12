package scopemigrate

import (
	domain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/resource"
	grantpo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/permissiongrant"
	"testing"
	"time"
)

func TestReportPreservesGrantEvidenceWithoutInferringLegacyDataAccess(t *testing.T) {
	state, qs, input := fixture()
	grant, err := domain.New(2, resource.NewResourceID(4), "qs:actor:collection:testees", "read", "user:9")
	if err != nil {
		t.Fatal(err)
	}
	grant.ID = 10
	row, err := (grantpo.Mapper{}).ToPO(&grant)
	if err != nil {
		t.Fatal(err)
	}
	revoked := *row
	revoked.ID = 11
	now := time.Now()
	revoked.RevokedAt = &now
	state.Grants = []grantpo.GrantPO{*row, revoked}
	plan := Build(state, qs, input)
	if err := plan.Validate(); err != nil {
		t.Fatal(err)
	}
	change := plan.Changes[0]
	if change.BeforeScope != "unconfigured" || len(change.Permissions) != 1 || change.Permissions[0].GrantID != "10" || change.Permissions[0].Resource != "qs:actor:collection:testees" || change.Permissions[0].Action != "read" {
		t.Fatalf("invalid report %+v", change)
	}
	state.Grants[0].ConstraintSet = `{"version":1,"all_of":[{"invalid":true}]}`
	if Build(state, qs, input).Validate() == nil {
		t.Fatal("invalid active authorization omitted from report")
	}
}
