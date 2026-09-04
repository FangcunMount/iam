package signingkey

import (
	"strings"
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// Key is the non-sensitive domain identity and lifecycle of a signing key.
// Cryptographic material and JWK/JWKS representations belong to adapters.
type Key struct {
	Kid       string
	Algorithm string
	Status    Status
	NotBefore *time.Time
	NotAfter  *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type KeyOption func(*Key)

func NewKey(kid, algorithm string, opts ...KeyOption) *Key {
	now := time.Now()
	key := &Key{
		Kid:       strings.TrimSpace(kid),
		Algorithm: strings.TrimSpace(algorithm),
		Status:    StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	for _, opt := range opts {
		opt(key)
	}
	return key
}

// RestoreKey rehydrates persisted lifecycle facts without replaying transitions.
// The persistence adapter must call Validate after combining them with key material.
func RestoreKey(
	kid, algorithm string,
	status Status,
	notBefore, notAfter *time.Time,
	createdAt, updatedAt time.Time,
) *Key {
	return &Key{
		Kid:       strings.TrimSpace(kid),
		Algorithm: strings.TrimSpace(algorithm),
		Status:    status,
		NotBefore: notBefore,
		NotAfter:  notAfter,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
}

func WithNotBefore(t time.Time) KeyOption {
	return func(k *Key) { k.NotBefore = &t }
}

func WithNotAfter(t time.Time) KeyOption {
	return func(k *Key) { k.NotAfter = &t }
}

func WithStatus(status Status) KeyOption {
	return func(k *Key) { k.Status = status }
}

func (k *Key) IsActive() bool  { return k != nil && k.Status == StatusActive }
func (k *Key) IsGrace() bool   { return k != nil && k.Status == StatusGrace }
func (k *Key) IsRetired() bool { return k != nil && k.Status == StatusRetired }

func (k *Key) CanSign() bool { return k.CanSignAt(time.Now()) }

func (k *Key) CanVerify() bool { return k.CanVerifyAt(time.Now()) }

func (k *Key) CanSignAt(now time.Time) bool {
	return k != nil && k.Status.CanSign() && k.IsValidAt(now)
}

func (k *Key) CanVerifyAt(now time.Time) bool {
	return k != nil && k.Status.CanVerify() && k.IsValidAt(now)
}

func (k *Key) ShouldPublish() bool { return k.ShouldPublishAt(time.Now()) }

func (k *Key) ShouldPublishAt(now time.Time) bool {
	return k != nil && k.Status.CanVerify() && k.IsValidAt(now)
}

func (k *Key) IsExpired(now time.Time) bool {
	return k != nil && k.NotAfter != nil && !now.Before(*k.NotAfter)
}

func (k *Key) IsNotYetValid(now time.Time) bool {
	return k != nil && k.NotBefore != nil && now.Before(*k.NotBefore)
}

func (k *Key) IsValidAt(now time.Time) bool {
	return k != nil && !k.IsExpired(now) && !k.IsNotYetValid(now)
}

// EnterGrace makes the old active key verification-only until graceUntil.
func (k *Key) EnterGrace(graceUntil, now time.Time) error {
	if k == nil {
		return errors.WithCode(code.ErrInvalidStateTransition, "signing key is required")
	}
	status, err := k.Status.EnterGrace()
	if err != nil {
		return err
	}
	if !graceUntil.After(now) {
		return errors.WithCode(code.ErrInvalidTimeRange, "grace period must end after transition time")
	}
	k.Status = status
	k.NotAfter = &graceUntil
	k.UpdatedAt = now
	return nil
}

// Retire ends verification eligibility after a grace key has expired.
func (k *Key) Retire(now time.Time) error {
	if k == nil {
		return errors.WithCode(code.ErrInvalidStateTransition, "signing key is required")
	}
	status, err := k.Status.Retire()
	if err != nil {
		return err
	}
	if !k.IsExpired(now) {
		return errors.WithCode(code.ErrInvalidStateTransition, "grace key cannot be retired before NotAfter")
	}
	k.Status = status
	k.UpdatedAt = now
	return nil
}

// ForceRetire is an emergency transition for non-active keys. An active key
// must first be replaced so the system never loses its only signer.
func (k *Key) ForceRetire(now time.Time) error {
	if k == nil {
		return errors.WithCode(code.ErrInvalidStateTransition, "signing key is required")
	}
	if k.IsActive() {
		return errors.WithCode(code.ErrInvalidStateTransition, "cannot force-retire the active signing key; activate a replacement first")
	}
	k.Status = StatusRetired
	k.UpdatedAt = now
	return nil
}

func (k *Key) Validate() error {
	if k == nil {
		return errors.WithCode(code.ErrInvalidKid, "signing key is required")
	}
	if strings.TrimSpace(k.Kid) == "" {
		return errors.WithCode(code.ErrInvalidKid, "kid cannot be empty")
	}
	if strings.TrimSpace(k.Algorithm) == "" {
		return errors.WithCode(code.ErrInvalidArgument, "signing algorithm cannot be empty")
	}
	switch k.Status {
	case StatusActive, StatusGrace, StatusRetired:
	default:
		return errors.WithCode(code.ErrInvalidStateTransition, "unknown signing key status")
	}
	if k.NotBefore != nil && k.NotAfter != nil && !k.NotAfter.After(*k.NotBefore) {
		return errors.WithCode(code.ErrInvalidTimeRange, "NotAfter must be after NotBefore")
	}
	return nil
}
