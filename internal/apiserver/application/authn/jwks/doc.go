// Package jwks provides Authn application use cases for JWKS publication and
// JWKS key administration.
//
// This package owns request/response DTOs and ports used by transport. Signing
// key lifecycle rules, PEM/RSA handling, JWK material construction, and cache
// snapshots are implemented by infra/token/keyset behind these ports.
//
// Authn domain packages must not depend on JWKS concepts. JWKS is a public
// token protocol boundary exposed through application and transport only.
package jwks
