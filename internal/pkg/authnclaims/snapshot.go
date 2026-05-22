package authnclaims

import (
	"encoding/json"
	"fmt"
)

// EncodeSnapshot 将 Principal 附加 claims 编码为 session/refresh 共用的字符串快照。
func EncodeSnapshot(in map[string]any) map[string]string {
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

// DecodeSnapshot 将字符串快照还原为 claims 映射。
func DecodeSnapshot(in map[string]string) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
