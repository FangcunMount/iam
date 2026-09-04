package authnclaims

import (
	"encoding/json"
	"fmt"
)

// jwtAttributeAllowlist 是 AccessToken attributes 的显式对外合同字段。
// 未知字段默认忽略；敏感 provider 凭据不得进入 allowlist。
var jwtAttributeAllowlist = map[string]struct{}{
	// 迁移窗口内保留 attributes.auth_time，供尚未切换的消费者双读。
	"auth_time": {},
}

var jwtAttributeDenyList = map[string]struct{}{
	"phone_number": {}, "wx_openid": {}, "wx_unionid": {},
	"wecom_user_id": {}, "wecom_open_user_id": {}, "wecom_state": {}, "wecom_corp_id": {},
	"auth_method": {}, "realm": {}, "login_identity_id": {},
	"tenant_domain": {}, "org_id": {}, "provider_raw": {},
}

// serviceAttributeAllowlist 是 ServiceToken attributes 的显式合同。
var serviceAttributeAllowlist = map[string]struct{}{
	"scope": {},
	"level": {},
}

// EncodeJWTAttributes 将已准入字段编码为 JWT attributes（allowlist）。
func EncodeJWTAttributes(in map[string]any) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if _, allowed := jwtAttributeAllowlist[k]; !allowed {
			continue
		}
		if _, denied := jwtAttributeDenyList[k]; denied {
			continue
		}
		if k == "" || v == nil {
			continue
		}
		out[k] = stringifyClaim(v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// EncodeServiceAttributes 将服务令牌 attributes 收敛到显式 allowlist。
func EncodeServiceAttributes(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if _, allowed := serviceAttributeAllowlist[k]; !allowed {
			continue
		}
		if k == "" || v == "" {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func stringifyClaim(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		if b, err := json.Marshal(t); err == nil {
			return string(b)
		}
		return fmt.Sprint(v)
	}
}
