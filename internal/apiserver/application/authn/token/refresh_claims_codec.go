package token

import tokendomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/token"

// NewDefaultRefreshClaimsCodec 创建默认 refresh claims 编解码器。
func NewDefaultRefreshClaimsCodec() RefreshClaimsCodec {
	return tokendomain.NewDefaultRefreshClaimsCodec()
}
