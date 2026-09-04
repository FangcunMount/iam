// Package keyset adapts signing-key domain facts to cryptographic material,
// persistence ports, JWS key lookup and public JWKS serialization.
//
// The signingkey domain owns the non-sensitive Key entity, validity interval,
// signing/verification eligibility and lifecycle transitions. This package
// embeds that entity only to attach a PublicJWK and to coordinate infrastructure
// work that cannot live in the domain:
//
//   - RSA key-pair generation and PEM private-key storage/resolution;
//   - repository-backed atomic activation of a candidate key;
//   - conversion between domain metadata, public JWK and JWS signing keys;
//   - deterministic JWKS JSON, ETag and in-process publication snapshots.
//
// KeyManager is an infrastructure coordinator because activation spans private
// material and a database transaction. It delegates state rules to the embedded
// signingkey.Key. KeySetBuilder publishes only active/grace public keys that are
// valid at the build time; it never exposes private material. JWSKeySourceAdapter
// supplies RSA signing or verification keys to the compact-JWS codec after the
// domain eligibility checks pass.
//
// JWKS publication is a public protocol boundary, not the signing-key aggregate.
// Application use cases are split accordingly: application/authn/signingkey
// orchestrates administration, while application/authn/jwks exposes public JWK
// Set publication and cache metadata.
package keyset
