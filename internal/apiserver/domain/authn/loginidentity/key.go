package loginidentity

import (
	"strings"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ProviderKey is the canonical key used to resolve one login identity.
type ProviderKey struct {
	Provider         Provider
	Realm            string
	Identifier       string
	GlobalIdentifier string
}

func NewProviderKey(provider Provider, realm, identifier string) ProviderKey {
	return ProviderKey{
		Provider:   provider,
		Realm:      strings.TrimSpace(realm),
		Identifier: strings.TrimSpace(identifier),
	}
}

func UsernameProviderKey(tenantID meta.ID, username string) ProviderKey {
	return NewProviderKey(ProviderUsername, UsernameRealm(tenantID), username)
}

func MockConsumerProviderKey(username string) ProviderKey {
	return NewProviderKey(ProviderUsername, RealmDefault, username)
}

func PhoneProviderKey(phone string) ProviderKey {
	return NewProviderKey(ProviderPhone, RealmGlobal, phone)
}

func WechatMinipProviderKey(appID, openID, unionID string) ProviderKey {
	key := NewProviderKey(ProviderWechatMinip, appID, openID)
	key.GlobalIdentifier = strings.TrimSpace(unionID)
	return key
}

func WecomProviderKey(corpID, userIDInWecom string) ProviderKey {
	return NewProviderKey(ProviderWecom, corpID, userIDInWecom)
}

func UsernameRealm(tenantID meta.ID) string {
	if tenantID.IsZero() {
		return RealmDefault
	}
	return tenantID.String()
}

func (k ProviderKey) IsValid() bool {
	return k.Provider.Validate() &&
		strings.TrimSpace(k.Realm) != "" &&
		strings.TrimSpace(k.Identifier) != ""
}
