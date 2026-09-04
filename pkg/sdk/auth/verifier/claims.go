package verifier

import (
	"time"

	"github.com/lestrrat-go/jwx/v2/jwt"
)

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
	var tenantRaw, orgRaw string
	if v, ok := token.Get("tenant_id"); ok {
		tenantRaw = claimString(v)
	}
	if v, ok := token.Get("org_id"); ok {
		orgRaw = claimString(v)
	}
	applyTenantAndOrg(claims, tenantRaw, orgRaw)
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
	if v, ok := token.Get("auth_time"); ok {
		switch typed := v.(type) {
		case float64:
			claims.AuthTime = time.Unix(int64(typed), 0).UTC()
		case int64:
			claims.AuthTime = time.Unix(typed, 0).UTC()
		case string:
			if parsed, err := time.Parse(time.RFC3339, typed); err == nil {
				claims.AuthTime = parsed.UTC()
			}
		}
	}
	claims.AuthenticatedAt = claims.AuthTime
	if v, ok := token.Get("attributes"); ok {
		if attrs, ok := v.(map[string]interface{}); ok {
			claims.Attributes = make(map[string]string, len(attrs))
			for k, val := range attrs {
				claims.Extra[k] = val
				claims.Attributes[k] = claimString(val)
			}
			if claims.AuthTime.IsZero() {
				if raw, ok := attrs["auth_time"]; ok {
					if text := claimString(raw); text != "" {
						if parsed, err := time.Parse(time.RFC3339, text); err == nil {
							claims.AuthTime = parsed.UTC()
						}
					}
				}
			}
		}
	}
	if claims.TokenType == "" {
		claims.TokenType = "access"
	}
	claims.AuthenticatedAt = claims.AuthTime

	return claims
}
