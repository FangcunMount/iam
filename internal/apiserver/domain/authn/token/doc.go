// Package token models AuthN tokens and their lifecycle services.
//
// AccessToken, RefreshToken, and ServiceToken are distinct domain objects that
// share token metadata but carry different invariants. TokenSetMinter,
// Refresher, Verifier, and Revoker express token lifecycle behavior; JWT/JWS
// encoding and Redis persistence are supplied through domain ports. The
// Session + TokenSet authentication result is owned by domain/authn/grant.
package token
