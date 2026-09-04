package signingkey

import "time"

// PublicJWK is the public projection included in signing-key administration responses.
// It can never carry private RSA parameters.
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

// ManagedKey is a non-sensitive signing-key lifecycle snapshot.
type ManagedKey struct {
	Kid       string
	Algorithm string
	Status    string
	JWK       PublicJWK
	NotBefore *time.Time
	NotAfter  *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}
