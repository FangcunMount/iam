package jwt

import (
	"github.com/FangcunMount/iam/v3/internal/pkg/authnclaims"
)

const (
	headerTypeJWT = "JWT"   // 令牌类型
	algRS256      = "RS256" // 算法
)

// Header 是 IAM access/service token 使用的 JOSE Header。
// 它包含了令牌类型、算法和签名密钥 ID。
type Header struct {
	Type      string `json:"typ"` // 令牌类型
	Algorithm string `json:"alg"` // 算法
	KeyID     string `json:"kid"` // 签名密钥 ID
}

// jwtAttributeEncoder 将 AccessTokenSubject 附加 claims 编码为 JWT attributes claim。
// 它负责将 AccessTokenSubject 附加 claims 编码为 JWT attributes claim。
type jwtAttributeEncoder struct{}

// newJWTAttributeEncoder 创建 JWT 属性编码器
func newJWTAttributeEncoder() jwtAttributeEncoder {
	// 创建 JWT 属性编码器
	return jwtAttributeEncoder{}
}

// EncodeAttributes 将 AccessTokenSubject 附加 claims 编码为 JWT attributes claim。
func (j *jwtAttributeEncoder) EncodeAttributes(in map[string]any) map[string]string {
	// 编码 JWT attributes claim
	return authnclaims.EncodeJWTAttributes(in)
}
