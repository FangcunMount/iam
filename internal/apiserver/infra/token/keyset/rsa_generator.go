package keyset

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	pkgauth "github.com/FangcunMount/iam/v3/pkg/auth"
)

// RSAKeyGenerator RSA 密钥生成器
// 实现 KeyGenerator 接口
type RSAKeyGenerator struct {
	// 默认密钥大小（位）
	defaultKeySize int
}

// NewRSAKeyGenerator 创建 RSA 密钥生成器
// 默认使用 2048 位密钥
func NewRSAKeyGenerator() *RSAKeyGenerator {
	return &RSAKeyGenerator{
		defaultKeySize: 2048,
	}
}

// NewRSAKeyGeneratorWithSize 创建指定密钥大小的 RSA 密钥生成器
// keySize: 密钥大小（位），推荐 2048 或 4096
func NewRSAKeyGeneratorWithSize(keySize int) *RSAKeyGenerator {
	return &RSAKeyGenerator{
		defaultKeySize: keySize,
	}
}

// Ensure RSAKeyGenerator implements KeyGenerator
var _ KeyGenerator = (*RSAKeyGenerator)(nil)

// GenerateKeyPair 生成 RSA 密钥对
func (g *RSAKeyGenerator) GenerateKeyPair(ctx context.Context, algorithm, kid string) (*KeyPair, error) {
	// 验证算法
	if !IsSupportedAlgorithm(algorithm) {
		return nil, errors.WithCode(
			code.ErrUnsupportedKty,
			"unsupported algorithm: %s, supported: %s",
			algorithm,
			pkgauth.TokenProfileAlgorithm,
		)
	}

	// 根据算法确定密钥大小
	keySize := g.getKeySizeForAlgorithm(algorithm)

	// 生成 RSA 私钥
	privateKey, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return nil, errors.WithCode(
			code.ErrUnknown,
			"failed to generate RSA private key: %v",
			err,
		)
	}

	// 构建 PublicJWK
	publicJWK, err := g.buildPublicJWK(privateKey, algorithm, kid)
	if err != nil {
		return nil, err
	}

	// 验证生成的 JWK
	if err := publicJWK.Validate(); err != nil {
		return nil, errors.WithCode(
			code.ErrInvalidJWK,
			"generated JWK validation failed: %v",
			err,
		)
	}

	return &KeyPair{
		PrivateKey: privateKey,
		PublicJWK:  publicJWK,
	}, nil
}

// SupportedAlgorithms 返回支持的算法列表
func (g *RSAKeyGenerator) SupportedAlgorithms() []string {
	return []string{pkgauth.TokenProfileAlgorithm}
}

// IsSupportedAlgorithm 检查是否支持该算法
func IsSupportedAlgorithm(algorithm string) bool {
	return algorithm == pkgauth.TokenProfileAlgorithm
}

// getKeySizeForAlgorithm 根据算法获取推荐的密钥大小
func (g *RSAKeyGenerator) getKeySizeForAlgorithm(_ string) int {
	// RS256, RS384, RS512 都使用相同的密钥大小
	// 区别在于哈希算法（SHA-256, SHA-384, SHA-512）
	// 2048 位对所有算法都足够安全
	// 4096 位提供更高的安全性但性能较低
	return g.defaultKeySize
}

// buildPublicJWK 从 RSA 私钥构建 PublicJWK
func (g *RSAKeyGenerator) buildPublicJWK(privateKey *rsa.PrivateKey, alg, kid string) (PublicJWK, error) {
	// 获取公钥
	publicKey := &privateKey.PublicKey

	// 将 n (modulus) 编码为 base64url
	nBytes := publicKey.N.Bytes()
	n := base64.RawURLEncoding.EncodeToString(nBytes)

	// 将 e (exponent) 编码为 base64url
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes())

	return PublicJWK{
		Kty: "RSA",
		Use: "sig",
		Alg: alg,
		Kid: kid,
		N:   &n,
		E:   &e,
	}, nil
}

// GetKeySize 获取当前配置的密钥大小
func (g *RSAKeyGenerator) GetKeySize() int {
	return g.defaultKeySize
}

// ValidateKeySize 验证密钥大小是否合理
// RSA 密钥大小应该至少为 2048 位，推荐 2048 或 4096
func ValidateKeySize(keySize int) error {
	if keySize < 2048 {
		return fmt.Errorf("key size too small: %d, minimum is 2048", keySize)
	}
	if keySize%1024 != 0 {
		return fmt.Errorf("key size should be a multiple of 1024: %d", keySize)
	}
	if keySize > 8192 {
		return fmt.Errorf("key size too large: %d, maximum is 8192", keySize)
	}
	return nil
}

// GetAlgorithmInfo 获取算法信息（辅助方法）
// GetAlgorithmInfo 获取算法信息
func GetAlgorithmInfo(algorithm string) *AlgorithmInfo {
	switch algorithm {
	case pkgauth.TokenProfileAlgorithm:
		return &AlgorithmInfo{
			Algorithm:       pkgauth.TokenProfileAlgorithm,
			HashAlgorithm:   "SHA-256",
			RecommendedSize: 2048,
			MinimumSize:     2048,
			Description:     "RSA Signature with SHA-256",
		}
	default:
		return nil
	}
}

// AlgorithmInfo 算法信息
type AlgorithmInfo struct {
	Algorithm       string // 算法名称（RS256/RS384/RS512）
	HashAlgorithm   string // 哈希算法（SHA-256/SHA-384/SHA-512）
	RecommendedSize int    // 推荐密钥大小（位）
	MinimumSize     int    // 最小密钥大小（位）
	Description     string // 描述
}
