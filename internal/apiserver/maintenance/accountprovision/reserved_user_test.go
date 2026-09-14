package accountprovision

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	user "github.com/FangcunMount/iam/v5/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
)

type captureUser struct {
	user.Repository
	got meta.ID
}

func (r *captureUser) Create(_ context.Context, u *user.User) error { r.got = u.ID; return nil }
func TestReservedIDIsMaintenanceOnlyAndBoundToFingerprint(t *testing.T) {
	i := Input{RequestID: "reviewer-10002", ActorID: "10001", Username: "review@mfangcunmount.com", Name: "安全与产品审核员", Reason: "approved provisioning", Password: "Test-Password-Private-123"}
	if e := i.Validate(); e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal([]string{i.RequestID, i.ActorID, i.Username, i.Name, i.Reason})
	if i.fingerprint() != fmt.Sprintf("%x", sha256.Sum256(b)) {
		t.Fatal("legacy fingerprint changed")
	}
	old := i.fingerprint()
	i.UserID = "10002"
	if e := i.Validate(); e != nil || old == i.fingerprint() {
		t.Fatal("reserved ID not bound", e)
	}
	for _, id := range []string{"10001", "010002", "0", "-1", "abc"} {
		i.UserID = id
		if i.Validate() == nil {
			t.Fatal("invalid ID accepted", id)
		}
	}
	repo := &captureUser{}
	r := reservedUserRepository{Repository: repo, id: meta.ID(10002)}
	u, _ := user.NewUser("reviewer", meta.Phone{})
	if e := r.Create(context.Background(), u); e != nil || repo.got != 10002 {
		t.Fatal(e)
	}
	if e := r.Create(context.Background(), u); e == nil {
		t.Fatal("reassignment allowed")
	}
}
