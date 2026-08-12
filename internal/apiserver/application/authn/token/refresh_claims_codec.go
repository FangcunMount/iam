package token

import (
	"github.com/FangcunMount/iam/v3/internal/pkg/authnclaims"
)

// defaultRefreshClaimsCodec 使用 session/refresh 共用的快照规则。
type defaultRefreshClaimsCodec struct{}

// NewDefaultRefreshClaimsCodec 创建默认 refresh claims 编解码器。
func NewDefaultRefreshClaimsCodec() RefreshClaimsCodec {
	return defaultRefreshClaimsCodec{}
}

func (defaultRefreshClaimsCodec) Encode(in map[string]any) map[string]string {
	return authnclaims.EncodeSnapshot(in)
}

func (defaultRefreshClaimsCodec) Decode(in map[string]string) map[string]any {
	return authnclaims.DecodeSnapshot(in)
}

// normalizeRefreshClaimsCodec 规范化 refresh claims 编解码器。
func normalizeRefreshClaimsCodec(codec RefreshClaimsCodec) RefreshClaimsCodec {
	if codec == nil {
		return NewDefaultRefreshClaimsCodec()
	}
	return codec
}
