package loginidentity

import (
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type Option func(*LoginIdentity)

func WithID(id meta.ID) Option { return func(i *LoginIdentity) { i.ID = id } }

func WithStatus(status Status) Option { return func(i *LoginIdentity) { i.Status = status } }

func WithVerifiedAt(verifiedAt time.Time) Option {
	return func(i *LoginIdentity) { i.VerifiedAt = &verifiedAt }
}

func WithProfile(profile map[string]string) Option {
	return func(i *LoginIdentity) { i.Profile = cloneStringMap(profile) }
}

func WithMeta(metaData map[string]string) Option {
	return func(i *LoginIdentity) { i.Meta = cloneStringMap(metaData) }
}

func WithLinkedAt(linkedAt time.Time) Option {
	return func(i *LoginIdentity) { i.LinkedAt = linkedAt }
}

func NewUsernameIdentity(userID meta.ID, realm, username string, opts ...Option) (*LoginIdentity, error) {
	return newLoginIdentity(userID, ProviderUsername, normalizeRealm(realm), strings.TrimSpace(username), "", opts...)
}

func NewPhoneIdentity(userID meta.ID, phone string, opts ...Option) (*LoginIdentity, error) {
	return newLoginIdentity(userID, ProviderPhone, RealmGlobal, strings.TrimSpace(phone), "", opts...)
}

func NewWechatMinipIdentity(userID meta.ID, appID, openID, unionID string, opts ...Option) (*LoginIdentity, error) {
	return newLoginIdentity(userID, ProviderWechatMinip, strings.TrimSpace(appID), strings.TrimSpace(openID), strings.TrimSpace(unionID), opts...)
}

func NewWecomIdentity(userID meta.ID, corpID, userIDInWecom string, opts ...Option) (*LoginIdentity, error) {
	return newLoginIdentity(userID, ProviderWecom, strings.TrimSpace(corpID), strings.TrimSpace(userIDInWecom), "", opts...)
}

func newLoginIdentity(userID meta.ID, provider Provider, realm, identifier, globalIdentifier string, opts ...Option) (*LoginIdentity, error) {
	if userID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "user_id is required")
	}
	if !provider.Validate() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid login identity provider: %s", provider)
	}
	realm = strings.TrimSpace(realm)
	if realm == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "realm is required")
	}
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "identifier is required")
	}

	now := time.Now()
	identity := &LoginIdentity{
		UserID:           userID,
		Provider:         provider,
		Realm:            realm,
		Identifier:       identifier,
		GlobalIdentifier: strings.TrimSpace(globalIdentifier),
		Status:           StatusActive,
		LinkedAt:         now,
		Profile:          map[string]string{},
		Meta:             map[string]string{},
	}
	for _, opt := range opts {
		opt(identity)
	}
	if identity.LinkedAt.IsZero() {
		identity.LinkedAt = now
	}
	if identity.Profile == nil {
		identity.Profile = map[string]string{}
	}
	if identity.Meta == nil {
		identity.Meta = map[string]string{}
	}
	if !identity.Status.Validate() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid login identity status: %s", identity.Status)
	}
	return identity, nil
}

func cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
