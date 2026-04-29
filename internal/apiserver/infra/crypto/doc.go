// Package crypto provides generic authentication cryptography adapters.
//
// Signing-key, RSA, PEM, and JWKS-specific infrastructure lives in
// internal/apiserver/infra/token/keyset. This package is kept for reusable
// cryptographic adapters such as password hashing and secret encryption.
package crypto
