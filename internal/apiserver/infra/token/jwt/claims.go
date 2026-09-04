package jwt

const headerTypeJWT = "JWT" // 令牌类型

// Header 是 IAM access/service token 使用的 JOSE Header。
// 它包含了令牌类型、算法和签名密钥 ID。
type Header struct {
	Type      string `json:"typ"` // 令牌类型
	Algorithm string `json:"alg"` // 算法
	KeyID     string `json:"kid"` // 签名密钥 ID
}
