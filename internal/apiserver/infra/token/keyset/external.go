package jwks

import (
	"context"
)

// ================== External Service Interfaces (Driven Ports) ==================
// 定义领域模型所依赖的外部服务接口，由基础设施层提供实现

// KeyPair 密钥对（私钥 + 公钥）
// 由 KeyGenerator 生成，返回给领域服务
type KeyPair struct {
	// PrivateKey 私钥（crypto.PrivateKey 类型）
	// RSA: *rsa.PrivateKey
	// EC: *ecdsa.PrivateKey
	// OKP: ed25519.PrivateKey
	PrivateKey any

	// PublicJWK 公钥的 JWK 表示
	PublicJWK PublicJWK
}

// KeyGenerator 密钥生成器接口
// 负责生成密钥对（RSA/EC/OKP）
// 可以有多种实现：RSA 2048/4096、EC P-256/P-384、Ed25519 等
type KeyGenerator interface {
	// GenerateKeyPair 生成密钥对
	// alg: 签名算法（RS256/RS384/RS512/ES256/ES384/ES512/EdDSA）
	// kid: 密钥 ID（由调用方提供，通常是 UUID）
	// 返回：KeyPair（包含私钥和公钥 JWK）
	GenerateKeyPair(ctx context.Context, alg string, kid string) (*KeyPair, error)

	// SupportedAlgorithms 返回支持的算法列表
	// 例如：["RS256", "RS384", "RS512"]
	SupportedAlgorithms() []string
}

// PrivateKeyStorage 私钥存储接口
// 负责私钥的持久化和删除操作
// 可以有多种实现：PEM 文件、KMS（AWS/GCP/Azure）、HSM 等
type PrivateKeyStorage interface {
	// SavePrivateKey 保存私钥
	// kid: 密钥 ID
	// privateKey: 私钥对象（*rsa.PrivateKey / *ecdsa.PrivateKey / ed25519.PrivateKey）
	// alg: 算法（RS256/ES256/EdDSA 等）
	SavePrivateKey(ctx context.Context, kid string, privateKey any, alg string) error

	// DeletePrivateKey 删除私钥
	// 当密钥被彻底退役时调用
	DeletePrivateKey(ctx context.Context, kid string) error

	// KeyExists 检查私钥是否存在
	KeyExists(ctx context.Context, kid string) (bool, error)
}

// PrivateKeyResolver 私钥解析器
// 签名侧拿"私钥句柄"的抽象；开发期可 PEM，生产期 KMS/HSM
type PrivateKeyResolver interface {
	// ResolveSigningKey 解析签名密钥
	// kid: 密钥 ID
	// alg: 算法
	// 返回：私钥对象（any 类型，实际是 crypto.PrivateKey）
	ResolveSigningKey(ctx context.Context, kid, alg string) (any, error)
}

// KeySetReader 密钥集读取器
// 对外发布用：供应用层生成 /.well-known/jwks.json
type KeySetReader interface {
	// CurrentJWKS 获取当前 JWKS JSON
	CurrentJWKS(ctx context.Context) (jwksJSON []byte, tag CacheTag, err error)

	// ActiveKeyMeta 获取当前激活的密钥元信息
	// 用于签名器或健康检查
	ActiveKeyMeta(ctx context.Context) (kid string, alg string, err error)
}
