package jwt

import (
	"github.com/FangcunMount/iam/v2/internal/pkg/authnclaims"
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

// jwtAttributeEncoder 将 AccessTokenSubject 附加 claims 编码为 JWT attributes claim。
type jwtAttributeEncoder struct{}

func newJWTAttributeEncoder() jwtAttributeEncoder {
	return jwtAttributeEncoder{}
}

func (jwtAttributeEncoder) EncodeAttributes(in map[string]any) map[string]string {
	return authnclaims.EncodeJWTAttributes(in)
}
