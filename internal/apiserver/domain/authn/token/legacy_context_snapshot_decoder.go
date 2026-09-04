package token

import "github.com/FangcunMount/iam/v3/internal/pkg/authnclaims"

type legacyAuthenticationContextSnapshotDecoder struct{}

// NewLegacyAuthenticationContextSnapshotDecoder 创建历史认证上下文快照解码器。
func NewLegacyAuthenticationContextSnapshotDecoder() LegacyAuthenticationContextSnapshotDecoder {
	return legacyAuthenticationContextSnapshotDecoder{}
}

func (legacyAuthenticationContextSnapshotDecoder) Decode(in map[string]string) map[string]any {
	return authnclaims.DecodeSnapshot(in)
}

func normalizeLegacyContextDecoder(decoder LegacyAuthenticationContextSnapshotDecoder) LegacyAuthenticationContextSnapshotDecoder {
	if decoder == nil {
		return NewLegacyAuthenticationContextSnapshotDecoder()
	}
	return decoder
}
