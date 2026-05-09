package loginidentity

import (
	"strings"
	"time"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// LoginIdentity binds an IAM user to a concrete login identifier.
type LoginIdentity struct {
	ID               meta.ID
	UserID           meta.ID
	Provider         Provider
	Realm            string
	Identifier       string
	GlobalIdentifier string
	Status           Status
	VerifiedAt       *time.Time
	LinkedAt         time.Time
	Profile          map[string]string
	Meta             map[string]string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (i *LoginIdentity) UniqueKey() (Provider, string, string) {
	if i == nil {
		return "", "", ""
	}
	return i.Provider, i.Realm, i.Identifier
}

func (i *LoginIdentity) IsActive() bool {
	return i != nil && i.Status == StatusActive
}

func (i *LoginIdentity) VerifyAt(t time.Time) {
	i.VerifiedAt = &t
}

func (i *LoginIdentity) Activate() { i.Status = StatusActive }

func (i *LoginIdentity) Disable() { i.Status = StatusDisabled }

func normalizeRealm(realm string) string {
	realm = strings.TrimSpace(realm)
	if realm == "" {
		return RealmDefault
	}
	return realm
}
