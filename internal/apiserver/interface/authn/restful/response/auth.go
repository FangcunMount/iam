package response

import "time"

// TokenPair 令牌对
type TokenPair struct {
	AccessToken  string `json:"access_token"`            // 访问令牌
	TokenType    string `json:"token_type"`              // 令牌类型（Bearer）
	ExpiresIn    int64  `json:"expires_in"`              // 过期时间（秒）
	RefreshToken string `json:"refresh_token,omitempty"` // 刷新令牌（可选）
}

// TokenVerifyResponse 验证令牌响应
type TokenVerifyResponse struct {
	Valid  bool         `json:"valid"`            // 令牌是否有效
	Claims *TokenClaims `json:"claims,omitempty"` // 令牌声明（如果有效）
}

// TokenClaims JWT 声明
type TokenClaims struct {
	UserID     string            `json:"user_id"`             // 用户 ID
	AccountID  string            `json:"account_id"`          // 账户 ID
	TenantID   *int64            `json:"tenant_id,omitempty"` // 租户 ID（可选）
	Issuer     string            `json:"issuer"`              // 签发者
	IssuedAt   time.Time         `json:"issued_at"`           // 签发时间
	ExpiresAt  time.Time         `json:"expires_at"`          // 过期时间
	JTI        string            `json:"jti,omitempty"`       // JWT ID（可选）
	KID        string            `json:"kid,omitempty"`       // Key ID（可选）
	Amr        []string          `json:"amr,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// MessageResponse 通用消息响应
type MessageResponse struct {
	Message string `json:"message"`
}

// JWKSet JWKS 公钥集
type JWKSet struct {
	Keys []JWK `json:"keys"`
}

// JWK JSON Web Key
type JWK struct {
	Kty string `json:"kty"`           // Key Type (RSA, EC, etc.)
	Kid string `json:"kid"`           // Key ID
	Use string `json:"use,omitempty"` // Public Key Use (sig, enc)
	Alg string `json:"alg,omitempty"` // Algorithm
	N   string `json:"n,omitempty"`   // RSA Modulus
	E   string `json:"e,omitempty"`   // RSA Exponent
	Crv string `json:"crv,omitempty"` // EC Curve
	X   string `json:"x,omitempty"`   // EC X Coordinate
	Y   string `json:"y,omitempty"`   // EC Y Coordinate
}
