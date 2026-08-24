package externalidentity

import (
	"fmt"
	"strings"
	"time"
)

// Provider identifies an external identity provider supported by IAM.
type Provider string

const (
	ProviderWechatMinip Provider = "wechat_minip"
	ProviderWechatOpen  Provider = "wechat_open"
	ProviderWecom       Provider = "wecom"
)

// Validate reports whether the provider is supported by the internal resolver.
func (p Provider) Validate() bool {
	switch p {
	case ProviderWechatMinip, ProviderWechatOpen, ProviderWecom:
		return true
	default:
		return false
	}
}

// IdentifierKind names a provider identifier without exposing provider SDK types.
type IdentifierKind string

const (
	IdentifierOpenID     IdentifierKind = "open_id"
	IdentifierUnionID    IdentifierKind = "union_id"
	IdentifierUserID     IdentifierKind = "user_id"
	IdentifierOpenUserID IdentifierKind = "open_user_id"
)

func (k IdentifierKind) validate() bool {
	switch k {
	case IdentifierOpenID, IdentifierUnionID, IdentifierUserID, IdentifierOpenUserID:
		return true
	default:
		return false
	}
}

// Identifier is a normalized provider identifier.
type Identifier struct {
	kind  IdentifierKind
	value string
}

// NewIdentifier constructs a normalized identifier.
func NewIdentifier(kind IdentifierKind, value string) (Identifier, error) {
	value = strings.TrimSpace(value)
	if !kind.validate() {
		return Identifier{}, fmt.Errorf("unsupported external identity identifier kind: %q", kind)
	}
	if value == "" {
		return Identifier{}, fmt.Errorf("external identity identifier value is required")
	}
	return Identifier{kind: kind, value: value}, nil
}

func (i Identifier) Kind() IdentifierKind { return i.kind }
func (i Identifier) Value() string        { return i.value }

// ExternalIdentity is a request-scoped proof returned by an external provider.
// It deliberately contains no IAM user/login identity and no reusable credential.
type ExternalIdentity struct {
	provider    Provider
	realm       string
	identifiers []Identifier
	verifiedAt  time.Time
}

// New constructs an immutable external identity and enforces provider invariants.
func New(provider Provider, realm string, identifiers []Identifier, verifiedAt time.Time) (ExternalIdentity, error) {
	realm = strings.TrimSpace(realm)
	if !provider.Validate() {
		return ExternalIdentity{}, fmt.Errorf("unsupported external identity provider: %q", provider)
	}
	if realm == "" {
		return ExternalIdentity{}, fmt.Errorf("external identity realm is required")
	}
	if verifiedAt.IsZero() {
		return ExternalIdentity{}, fmt.Errorf("external identity verified_at is required")
	}

	normalized := make([]Identifier, 0, len(identifiers))
	byKind := make(map[IdentifierKind]string, len(identifiers))
	for _, identifier := range identifiers {
		value := strings.TrimSpace(identifier.value)
		if value == "" {
			continue
		}
		if !identifier.kind.validate() {
			return ExternalIdentity{}, fmt.Errorf("unsupported external identity identifier kind: %q", identifier.kind)
		}
		if previous, exists := byKind[identifier.kind]; exists {
			if previous == value {
				continue
			}
			return ExternalIdentity{}, fmt.Errorf("multiple values for external identity identifier kind: %q", identifier.kind)
		}
		if !providerAllowsIdentifier(provider, identifier.kind) {
			return ExternalIdentity{}, fmt.Errorf("identifier kind %q is not valid for provider %q", identifier.kind, provider)
		}
		byKind[identifier.kind] = value
		normalized = append(normalized, Identifier{kind: identifier.kind, value: value})
	}

	if err := validateRequiredIdentifiers(provider, byKind); err != nil {
		return ExternalIdentity{}, err
	}

	return ExternalIdentity{
		provider:    provider,
		realm:       realm,
		identifiers: normalized,
		verifiedAt:  verifiedAt,
	}, nil
}

func providerAllowsIdentifier(provider Provider, kind IdentifierKind) bool {
	switch provider {
	case ProviderWechatMinip, ProviderWechatOpen:
		return kind == IdentifierOpenID || kind == IdentifierUnionID
	case ProviderWecom:
		return kind == IdentifierUserID || kind == IdentifierOpenUserID
	default:
		return false
	}
}

func validateRequiredIdentifiers(provider Provider, identifiers map[IdentifierKind]string) error {
	switch provider {
	case ProviderWechatMinip, ProviderWechatOpen:
		if identifiers[IdentifierOpenID] == "" {
			return fmt.Errorf("open_id is required for provider %q", provider)
		}
	case ProviderWecom:
		if identifiers[IdentifierUserID] == "" && identifiers[IdentifierOpenUserID] == "" {
			return fmt.Errorf("user_id or open_user_id is required for provider %q", provider)
		}
	}
	return nil
}

func (i ExternalIdentity) Provider() Provider    { return i.provider }
func (i ExternalIdentity) Realm() string         { return i.realm }
func (i ExternalIdentity) VerifiedAt() time.Time { return i.verifiedAt }

// Identifiers returns a defensive copy.
func (i ExternalIdentity) Identifiers() []Identifier {
	return append([]Identifier(nil), i.identifiers...)
}

// Identifier returns a normalized identifier by kind.
func (i ExternalIdentity) Identifier(kind IdentifierKind) (string, bool) {
	for _, identifier := range i.identifiers {
		if identifier.kind == kind {
			return identifier.value, true
		}
	}
	return "", false
}
