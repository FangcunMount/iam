package request

import (
	"time"
)

// CreateKeyRequest 创建密钥请求
type CreateKeyRequest struct {
	Algorithm string     `json:"algorithm" binding:"required,oneof=RS256 RS384 RS512"` // 签名算法
	NotBefore *time.Time `json:"notBefore,omitempty"`                                  // 生效时间（可选）
	NotAfter  *time.Time `json:"notAfter,omitempty"`                                   // 过期时间（可选）
}
