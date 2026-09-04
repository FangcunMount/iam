// Package signingkey owns non-sensitive signing-key lifecycle rules.
// Cryptographic key material, wire serialization and persistence remain adapters.
package signingkey

import (
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// Status is the IAM lifecycle state of a signing key; it is not a public key-set field.
type Status uint8

const (
	StatusActive Status = iota + 1
	StatusGrace
	StatusRetired
)

func (s Status) String() string {
	switch s {
	case StatusActive:
		return "active"
	case StatusGrace:
		return "grace"
	case StatusRetired:
		return "retired"
	default:
		return "unknown"
	}
}

func (s Status) CanSign() bool {
	return s == StatusActive
}

func (s Status) CanVerify() bool {
	return s == StatusActive || s == StatusGrace
}

func (s Status) EnterGrace() (Status, error) {
	if s != StatusActive {
		return s, errors.WithCode(code.ErrInvalidStateTransition, "can only enter grace period from active state")
	}
	return StatusGrace, nil
}

func (s Status) Retire() (Status, error) {
	if s != StatusGrace {
		return s, errors.WithCode(code.ErrInvalidStateTransition, "can only retire from grace period")
	}
	return StatusRetired, nil
}

// RotationPolicy constrains automatic signing-key rotation and public overlap.
type RotationPolicy struct {
	RotationInterval   time.Duration
	GracePeriod        time.Duration
	MaxPublishableKeys int
}

func DefaultRotationPolicy() RotationPolicy {
	return RotationPolicy{
		RotationInterval:   30 * 24 * time.Hour,
		GracePeriod:        7 * 24 * time.Hour,
		MaxPublishableKeys: 3,
	}
}

func (p RotationPolicy) Validate() error {
	if p.RotationInterval <= 0 {
		return errors.WithCode(code.ErrInvalidRotationInterval, "rotation interval must be positive")
	}
	if p.GracePeriod <= 0 {
		return errors.WithCode(code.ErrInvalidGracePeriod, "grace period must be positive")
	}
	if p.MaxPublishableKeys < 2 {
		return errors.WithCode(code.ErrInvalidMaxKeys, "max keys must be at least 2")
	}
	if p.GracePeriod >= p.RotationInterval {
		return errors.WithCode(code.ErrGracePeriodTooLong, "grace period must be shorter than rotation interval")
	}
	return nil
}
