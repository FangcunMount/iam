package token

import (
	"encoding/json"
	"fmt"
)

// stringClaimMapper 字符串声明映射器
type stringClaimMapper struct{}

// NewStringClaimMapper 创建字符串声明映射器
func NewStringClaimMapper() ClaimMapper {
	return stringClaimMapper{}
}

// Encode 编码声明
func (stringClaimMapper) Encode(in map[string]any) map[string]string {
	return stringifyClaims(in)
}

// Decode 解码声明
func (stringClaimMapper) Decode(in map[string]string) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// normalizeClaimMapper 规范化声明映射器
func normalizeClaimMapper(mapper ClaimMapper) ClaimMapper {
	if mapper == nil {
		return stringClaimMapper{}
	}
	return mapper
}

// stringifyClaims 将任意映射转换为字符串映射
func stringifyClaims(in map[string]any) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
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
