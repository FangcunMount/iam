package jwt

import (
	"encoding/json"
	"fmt"
)

const (
	headerTypeJWT = "JWT"
	algRS256      = "RS256"
)

// Header 是 IAM access/service token 使用的 JOSE Header。
type Header struct {
	Type      string `json:"typ"`
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
}

// ClaimsMapper 将应用层 Principal.Claims 映射到 JWT private claims。
type ClaimsMapper struct{}

func NewClaimsMapper() ClaimsMapper {
	return ClaimsMapper{}
}

var jwtReservedClaimKeys = map[string]struct{}{
	"token_type": {}, "user_id": {}, "login_identity_id": {}, "org_id": {}, "tenant_id": {},
	"jti": {}, "sub": {}, "iss": {}, "aud": {}, "exp": {}, "iat": {}, "nbf": {},
	"amr": {}, "attributes": {}, "audience": {}, "kid": {}, "alg": {}, "typ": {},
}

func (ClaimsMapper) Encode(in map[string]any) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if _, reserved := jwtReservedClaimKeys[k]; reserved {
			continue
		}
		if v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			out[k] = t
		case fmt.Stringer:
			out[k] = t.String()
		default:
			if b, err := json.Marshal(t); err == nil {
				out[k] = string(b)
			} else {
				out[k] = fmt.Sprint(v)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (ClaimsMapper) Decode(in map[string]string) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
