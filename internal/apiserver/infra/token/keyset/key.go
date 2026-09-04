package keyset

import (
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	signingkeydomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/signingkey"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

type KeyStatus = signingkeydomain.Status

const (
	KeyActive  = signingkeydomain.StatusActive
	KeyGrace   = signingkeydomain.StatusGrace
	KeyRetired = signingkeydomain.StatusRetired
)

type RotationPolicy = signingkeydomain.RotationPolicy

func DefaultRotationPolicy() RotationPolicy { return signingkeydomain.DefaultRotationPolicy() }

type PublicJWK struct {
	Kty string  `json:"kty"`
	Use string  `json:"use"`
	Alg string  `json:"alg"`
	Kid string  `json:"kid"`
	N   *string `json:"n,omitempty"`
	E   *string `json:"e,omitempty"`
	Crv *string `json:"crv,omitempty"`
	X   *string `json:"x,omitempty"`
	Y   *string `json:"y,omitempty"`
}

func (p *PublicJWK) Validate() error {
	if p.Kid == "" {
		return errors.WithCode(code.ErrInvalidKid, "kid cannot be empty")
	}
	if p.Kty == "" {
		return errors.WithCode(code.ErrInvalidJWK, "kty cannot be empty")
	}
	if p.Use != "sig" {
		return errors.WithCode(code.ErrInvalidJWKUse, "use must be 'sig'")
	}
	if p.Alg == "" {
		return errors.WithCode(code.ErrInvalidJWKAlg, "alg cannot be empty")
	}
	switch p.Kty {
	case "RSA":
		if p.N == nil || p.E == nil {
			return errors.WithCode(code.ErrMissingRSAParams, "n and e are required for RSA")
		}
	case "EC":
		if p.Crv == nil || p.X == nil || p.Y == nil {
			return errors.WithCode(code.ErrMissingECParams, "crv, x, y are required for EC")
		}
	case "OKP":
		if p.Crv == nil || p.X == nil {
			return errors.WithCode(code.ErrMissingOKPParams, "crv, x are required for OKP")
		}
	default:
		return errors.WithCode(code.ErrUnsupportedKty, "unsupported key type")
	}
	return nil
}

type JWKS struct {
	Keys []PublicJWK `json:"keys"`
}

func (j *JWKS) Validate() error {
	if len(j.Keys) == 0 {
		return errors.WithCode(code.ErrEmptyJWKS, "JWKS cannot be empty")
	}
	for i, key := range j.Keys {
		if err := key.Validate(); err != nil {
			return errors.Wrapf(err, "JWKS validation failed at index %d", i)
		}
	}
	return nil
}

func (j *JWKS) FindByKid(kid string) *PublicJWK {
	for i := range j.Keys {
		if j.Keys[i].Kid == kid {
			return &j.Keys[i]
		}
	}
	return nil
}

func (j *JWKS) Count() int {
	return len(j.Keys)
}

func (j *JWKS) IsEmpty() bool {
	return len(j.Keys) == 0
}

type CacheTag struct {
	ETag         string
	LastModified time.Time
}

func (c *CacheTag) IsZero() bool {
	return c.ETag == "" && c.LastModified.IsZero()
}

func (c *CacheTag) Matches(other CacheTag) bool {
	return c.ETag == other.ETag
}

type Key struct {
	signingkeydomain.Key
	JWK PublicJWK
}

func NewKey(kid string, jwk PublicJWK, opts ...KeyOption) *Key {
	return &Key{Key: *signingkeydomain.NewKey(kid, jwk.Alg, opts...), JWK: jwk}
}

type KeyOption = signingkeydomain.KeyOption

func WithNotBefore(t time.Time) KeyOption { return signingkeydomain.WithNotBefore(t) }

func WithNotAfter(t time.Time) KeyOption { return signingkeydomain.WithNotAfter(t) }

func WithStatus(status KeyStatus) KeyOption { return signingkeydomain.WithStatus(status) }

func RestoreKey(
	kid string,
	algorithm string,
	jwk PublicJWK,
	status KeyStatus,
	notBefore, notAfter *time.Time,
	createdAt, updatedAt time.Time,
) *Key {
	return &Key{
		Key: *signingkeydomain.RestoreKey(
			kid, algorithm, status, notBefore, notAfter, createdAt, updatedAt,
		),
		JWK: jwk,
	}
}

func (k *Key) Validate() error {
	if k == nil {
		return errors.WithCode(code.ErrInvalidKid, "signing key is required")
	}
	if err := k.Key.Validate(); err != nil {
		return err
	}
	if err := k.JWK.Validate(); err != nil {
		return err
	}
	if k.JWK.Kid != k.Kid {
		return errors.WithCode(code.ErrKidMismatch, "key.Kid and JWK.Kid must be equal")
	}
	if k.JWK.Alg != k.Algorithm {
		return errors.WithCode(code.ErrInvalidJWKAlg, "key algorithm and JWK alg must be equal")
	}
	return nil
}

type SnapshotStatus struct {
	Cached        bool
	KeyCount      int
	CacheTag      CacheTag
	LastBuildTime *time.Time
}
