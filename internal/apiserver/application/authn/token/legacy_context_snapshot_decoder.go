package token

import tokendomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/token"

// NewLegacyAuthenticationContextSnapshotDecoder 创建迁移前 refresh 认证上下文解码器。
func NewLegacyAuthenticationContextSnapshotDecoder() LegacyAuthenticationContextSnapshotDecoder {
	return tokendomain.NewLegacyAuthenticationContextSnapshotDecoder()
}
