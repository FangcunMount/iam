package authnclaims

import (
	"encoding/json"
	"fmt"
)

var jwtReservedClaimKeys = map[string]struct{}{
	"token_type": {}, "user_id": {}, "login_identity_id": {}, "org_id": {}, "tenant_id": {},
	"jti": {}, "sub": {}, "iss": {}, "aud": {}, "exp": {}, "iat": {}, "nbf": {},
	"amr": {}, "attributes": {}, "audience": {}, "kid": {}, "alg": {}, "typ": {},
}

// EncodeJWTAttributes 将 Principal 附加 claims 编码为 JWT attributes claim（过滤保留字段名）。
func EncodeJWTAttributes(in map[string]any) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if _, reserved := jwtReservedClaimKeys[k]; reserved {
			continue
		}
		if k == "" || v == nil {
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
