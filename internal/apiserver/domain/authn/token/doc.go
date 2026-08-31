// Package token models AuthN tokens and their lifecycle services.
//
// AccessToken, RefreshToken, and ServiceToken are distinct domain objects that
// share token metadata but carry different invariants. AuthenticationGrant
// makes the successful authentication result explicit as Session + TokenSet.
// Issuer, Refresher, Verifier, and Revoker express its lifecycle; JWT/JWS
// encoding and Redis persistence are supplied through domain ports.
package token
