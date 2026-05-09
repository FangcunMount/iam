package verifier

import "github.com/lestrrat-go/jwx/v2/jwt"

func extractClaims(token jwt.Token) *TokenClaims {
	claims := &TokenClaims{
		TokenID:   token.JwtID(),
		Subject:   token.Subject(),
		Issuer:    token.Issuer(),
		Audience:  token.Audience(),
		ExpiresAt: token.Expiration(),
		IssuedAt:  token.IssuedAt(),
		NotBefore: token.NotBefore(),
		Extra:     make(map[string]interface{}),
	}

	if v, ok := token.Get("user_id"); ok {
		claims.UserID = claimString(v)
	}
	if v, ok := token.Get("sid"); ok {
		claims.SessionID = claimString(v)
	}
	if v, ok := token.Get("tenant_id"); ok {
		claims.TenantID = claimString(v)
	}
	if v, ok := token.Get("login_identity_id"); ok {
		claims.LoginIdentityID = claimString(v)
	}
	if v, ok := token.Get("roles"); ok {
		claims.Roles = claimStringSlice(v)
	}
	if v, ok := token.Get("scopes"); ok {
		claims.Scopes = claimStringSlice(v)
	}
	if v, ok := token.Get("token_type"); ok {
		claims.TokenType = claimString(v)
	}
	if v, ok := token.Get("amr"); ok {
		claims.AMR = claimStringSlice(v)
	}
	if v, ok := token.Get("attributes"); ok {
		if attrs, ok := v.(map[string]interface{}); ok {
			for k, val := range attrs {
				claims.Extra[k] = val
			}
		}
	}

	return claims
}
