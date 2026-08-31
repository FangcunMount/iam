// Package grant establishes the complete online authentication result.
//
// A Grant combines the server-side Session with the user TokenSet returned to
// the caller. Issuer enforces authentication admission before either artifact
// is created, then coordinates Session creation, token minting, and refresh
// token persistence as one domain operation.
package grant
