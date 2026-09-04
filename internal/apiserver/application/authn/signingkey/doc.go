// Package signingkey orchestrates IAM signing-key administration and lifecycle.
//
// It deliberately does not own JWKS serialization. Public JWK Set publication
// remains in application/authn/jwks; private key material remains in infrastructure.
package signingkey
