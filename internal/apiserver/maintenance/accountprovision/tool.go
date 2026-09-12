// Package accountprovision opens explicitly reviewed username identities without any authorization grants.
package accountprovision

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	signup "github.com/FangcunMount/iam/v5/internal/apiserver/application/authn/signup"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authn/authentication"
	login "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authn/loginidentity"
	userdomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/identity/user"
	credentialrepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/credential"
	loginrepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/loginidentity"
	authnuow "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/authn"
	userrepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/user"
	database "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/FangcunMount/iam/v5/internal/pkg/requestctx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Input struct {
	RequestID string `json:"request_id"`
	ActorID   string `json:"actor_id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	Reason    string `json:"reason"`
	Password  string `json:"password"`
}
type Report struct {
	State           string `json:"state"`
	RequestID       string `json:"request_id"`
	Username        string `json:"username"`
	Fingerprint     string `json:"fingerprint"`
	UserID          string `json:"user_id,omitempty"`
	LoginIdentityID string `json:"login_identity_id,omitempty"`
	CredentialID    string `json:"credential_id,omitempty"`
}

func (i Input) Validate() error {
	actor, err := meta.ParseID(i.ActorID)
	if err != nil || actor <= 0 || actor.String() != i.ActorID {
		return fmt.Errorf("canonical actor ID required")
	}
	if _, err = login.NewUsernameProviderKey(i.Username); err != nil {
		return err
	}
	if _, err = userdomain.NewUser(i.Name, meta.Phone{}); err != nil {
		return err
	}
	for _, s := range []string{i.RequestID, i.Username, i.Name, i.Reason, i.Password} {
		if strings.TrimSpace(s) != s || s == "" {
			return fmt.Errorf("nonempty normalized account input required")
		}
	}
	if len(i.RequestID) > 64 || len([]rune(i.Reason)) > 500 || len(i.Password) < 20 || len(i.Password) > 128 {
		return fmt.Errorf("bounded request/reason and 20-128 byte generated password required")
	}
	return nil
}
func (i Input) fingerprint() string {
	// Never put password material or its digest in reports or identity metadata.
	raw, _ := json.Marshal([]string{i.RequestID, i.ActorID, i.Username, i.Name, i.Reason})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func authorize(ctx context.Context, db *gorm.DB, input Input) error {
	var count int64
	err := db.WithContext(ctx).Table("authz_assignments a").Joins("JOIN authz_roles r ON r.id=a.role_id").Joins("JOIN users u ON u.id=a.subject_id").Where("a.subject_type='user' AND a.subject_id=? AND a.deleted_at IS NULL AND r.deleted_at IS NULL AND r.name='platform_admin' AND r.management_protection='protected' AND u.deleted_at IS NULL AND u.status=1", input.ActorID).Count(&count).Error
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("active protected platform administrator required")
	}
	return nil
}
func inspect(ctx context.Context, db *gorm.DB, input Input, hasher authentication.PasswordHasher) (Report, error) {
	report := Report{State: "pending", RequestID: input.RequestID, Username: input.Username, Fingerprint: input.fingerprint()}
	var identity loginrepo.PO
	// Include deleted identities: a historical identifier is not an available new username.
	err := db.WithContext(ctx).Table(identity.TableName()).Where("provider='username' AND realm='default' AND identifier=?", input.Username).Take(&identity).Error
	if err == gorm.ErrRecordNotFound {
		return report, nil
	}
	if err != nil {
		return report, err
	}
	var marker map[string]string
	if json.Unmarshal(identity.Meta, &marker) != nil || marker["provision_request_id"] != input.RequestID || marker["provision_fingerprint"] != report.Fingerprint {
		return report, fmt.Errorf("username already exists outside this provisioning request")
	}
	if identity.DeletedAt != nil || identity.Status != "active" {
		return report, fmt.Errorf("provisioned identity changed state")
	}
	var owner struct {
		ID     uint64
		Name   string
		Status int
	}
	if err = db.WithContext(ctx).Table("users").Select("id,name,status").Where("id=? AND deleted_at IS NULL", identity.UserID).Take(&owner).Error; err != nil {
		return report, err
	}
	if owner.Status != 1 || owner.Name != input.Name {
		return report, fmt.Errorf("provisioned user facts changed")
	}
	var credential credentialrepo.V2PO
	if err = db.WithContext(ctx).Table(credential.TableName()).Where("login_identity_id=? AND type='password' AND deleted_at IS NULL", identity.ID).Take(&credential).Error; err != nil {
		return report, err
	}
	if credential.Status != "enabled" || !hasher.Verify(string(credential.Material), input.Password+hasher.Pepper()) {
		return report, fmt.Errorf("provisioned credential does not match the retained input; never reset automatically")
	}
	report.State = "historical_completed"
	report.UserID = identity.UserID.String()
	report.LoginIdentityID = identity.ID.String()
	report.CredentialID = credential.ID.String()
	return report, nil
}
func Preflight(ctx context.Context, db *gorm.DB, input Input, hasher authentication.PasswordHasher) (Report, error) {
	if err := input.Validate(); err != nil {
		return Report{}, err
	}
	if db == nil || hasher == nil {
		return Report{}, fmt.Errorf("account provisioning dependencies required")
	}
	if err := authorize(ctx, db, input); err != nil {
		return Report{}, err
	}
	return inspect(ctx, db, input, hasher)
}
func Apply(ctx context.Context, db *gorm.DB, input Input, hasher authentication.PasswordHasher, fingerprint string) (Report, error) {
	if err := input.Validate(); err != nil {
		return Report{}, err
	}
	if db == nil || hasher == nil || fingerprint != input.fingerprint() {
		return Report{}, fmt.Errorf("reviewed account fingerprint required")
	}
	actor, _ := meta.ParseID(input.ActorID)
	ctx = context.WithValue(ctx, requestctx.KeyUserID, actor)
	var report Report
	err := database.NewUnitOfWork(db).WithinTransaction(ctx, func(txctx context.Context) error {
		tx, err := database.RequireTx(txctx)
		if err != nil {
			return err
		}
		// Freeze the actor's authorization while opening the identity; no policy change is published.
		var policy struct{ PolicyVersion int64 }
		if err = tx.Table("authz_policy_versions").Clauses(clause.Locking{Strength: "SHARE"}).Where("id=1").Take(&policy).Error; err != nil {
			return err
		}
		if policy.PolicyVersion <= 0 {
			return fmt.Errorf("valid authorization policy required")
		}
		if err = authorize(txctx, tx, input); err != nil {
			return err
		}
		report, err = inspect(txctx, tx, input, hasher)
		if err != nil || report.State == "historical_completed" {
			return err
		}
		service := signup.NewSignupService(authnuow.NewUnitOfWork(db), hasher, nil, userrepo.NewRepository(db))
		result, err := service.SignUp(txctx, signup.SignupRequest{User: signup.SignupUserInput{Name: input.Name}, LoginIdentity: signup.UsernameLoginIdentityInput{Username: input.Username, Meta: map[string]string{"provision_request_id": input.RequestID, "provision_fingerprint": input.fingerprint(), "provision_reason": input.Reason}}, Credential: &signup.SignupCredentialInput{Password: &signup.PasswordCredentialInput{Plaintext: input.Password}}})
		if err != nil {
			return err
		}
		if result == nil || !result.IsNewUser || !result.IsNewLoginIdentity {
			return fmt.Errorf("concurrent username collision; no existing identity may be adopted")
		}
		report, err = inspect(txctx, tx, input, hasher)
		if err != nil {
			return err
		}
		var grants int64
		if err = tx.Table("authz_assignments").Where("subject_type='user' AND subject_id=? AND deleted_at IS NULL", result.UserID.String()).Count(&grants).Error; err != nil {
			return err
		}
		if grants != 0 {
			return fmt.Errorf("new identity unexpectedly has authorization grants")
		}
		return nil
	})
	return report, err
}
