package response

import (
	"time"

	jwksApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/jwks"
)

// KeyResponse 密钥响应
type KeyResponse struct {
	Kid       string             `json:"kid"`                 // 密钥 ID
	Status    string             `json:"status"`              // 密钥状态 (active, grace, retired)
	Algorithm string             `json:"algorithm"`           // 签名算法
	NotBefore *time.Time         `json:"notBefore,omitempty"` // 生效时间
	NotAfter  *time.Time         `json:"notAfter,omitempty"`  // 过期时间
	PublicJWK *jwksApp.PublicJWK `json:"publicJwk"`           // 公钥 JWK
	CreatedAt time.Time          `json:"createdAt"`           // 创建时间
	UpdatedAt time.Time          `json:"updatedAt,omitempty"` // 更新时间
}

// KeyInfo 密钥信息（列表项）
type KeyInfo struct {
	Kid       string             `json:"kid"`                 // 密钥 ID
	Status    string             `json:"status"`              // 密钥状态
	Algorithm string             `json:"algorithm"`           // 签名算法
	NotBefore *time.Time         `json:"notBefore,omitempty"` // 生效时间
	NotAfter  *time.Time         `json:"notAfter,omitempty"`  // 过期时间
	PublicJWK *jwksApp.PublicJWK `json:"publicJwk"`           // 公钥 JWK
	CreatedAt time.Time          `json:"createdAt"`           // 创建时间
	UpdatedAt time.Time          `json:"updatedAt,omitempty"` // 更新时间
}

// KeyListResponse 密钥列表响应
type KeyListResponse struct {
	Keys   []*KeyInfo `json:"keys"`   // 密钥列表
	Total  int64      `json:"total"`  // 总数
	Limit  int        `json:"limit"`  // 每页数量
	Offset int        `json:"offset"` // 偏移量
}

// PublishableKeyInfo 可发布的密钥信息
type PublishableKeyInfo struct {
	Kid       string             `json:"kid"`                 // 密钥 ID
	Status    string             `json:"status"`              // 密钥状态
	Algorithm string             `json:"algorithm"`           // 签名算法
	NotBefore *time.Time         `json:"notBefore,omitempty"` // 生效时间
	NotAfter  *time.Time         `json:"notAfter,omitempty"`  // 过期时间
	PublicJWK *jwksApp.PublicJWK `json:"publicJwk"`           // 公钥 JWK
}

// PublishableKeysResponse 可发布的密钥列表响应
type PublishableKeysResponse struct {
	Keys []*PublishableKeyInfo `json:"keys"` // 可发布的密钥列表
}

// CleanupResponse 清理响应
type CleanupResponse struct {
	DeletedCount int `json:"deletedCount"` // 删除的密钥数量
}
