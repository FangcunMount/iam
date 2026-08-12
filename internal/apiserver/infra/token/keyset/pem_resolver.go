package keyset

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// PEMPrivateKeyResolver 从 PEM 文件读取私钥
// 适用于开发环境和简单场景
// 生产环境建议使用 KMS/HSM
type PEMPrivateKeyResolver struct {
	// keysDir 私钥文件存储目录
	// 文件命名规则：{kid}.pem 或 key-{kid}.pem
	keysDir string
}

var _ PrivateKeyResolver = (*PEMPrivateKeyResolver)(nil)

// NewPEMPrivateKeyResolver 创建 PEM 私钥解析器
func NewPEMPrivateKeyResolver(keysDir string) PrivateKeyResolver {
	return &PEMPrivateKeyResolver{
		keysDir: keysDir,
	}
}

// ResolveSigningKey 解析私钥用于签名
func (r *PEMPrivateKeyResolver) ResolveSigningKey(ctx context.Context, kid, alg string) (any, error) {
	// 构建 PEM 文件路径
	pemPath := r.getPEMPath(kid)

	// 检查文件是否存在
	if _, err := os.Stat(pemPath); os.IsNotExist(err) {
		return nil, errors.WithCode(
			code.ErrKeyNotFound,
			"private key file not found: %s",
			pemPath,
		)
	}

	// 读取 PEM 文件
	pemData, err := os.ReadFile(pemPath)
	if err != nil {
		return nil, errors.WithCode(
			code.ErrUnknown,
			"failed to read private key file: %v",
			err,
		)
	}

	// 解析私钥
	privateKey, err := r.parsePrivateKey(pemData, alg)
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

// getPEMPath 获取 PEM 文件路径
// 支持两种命名规则：
// 1. {kid}.pem
// 2. key-{kid}.pem
func (r *PEMPrivateKeyResolver) getPEMPath(kid string) string {
	// 优先尝试 {kid}.pem
	path1 := filepath.Join(r.keysDir, kid+".pem")
	if _, err := os.Stat(path1); err == nil {
		return path1
	}

	// 尝试 key-{kid}.pem
	path2 := filepath.Join(r.keysDir, "key-"+kid+".pem")
	return path2
}

// parsePrivateKey 解析 PEM 格式的私钥
func (r *PEMPrivateKeyResolver) parsePrivateKey(pemData []byte, alg string) (any, error) {
	// 解码 PEM 块
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.WithCode(
			code.ErrInvalidJWK,
			"failed to decode PEM block",
		)
	}

	// 根据 PEM 块类型解析
	switch block.Type {
	case "RSA PRIVATE KEY":
		return r.parseRSAPrivateKey(block.Bytes, alg)
	case "PRIVATE KEY":
		return r.parsePKCS8PrivateKey(block.Bytes, alg)
	default:
		return nil, errors.WithCode(
			code.ErrUnsupportedKty,
			"unsupported PEM block type: %s (expected RSA PRIVATE KEY or PRIVATE KEY)",
			block.Type,
		)
	}
}

// parseRSAPrivateKey 解析 PKCS#1 格式的 RSA 私钥
func (r *PEMPrivateKeyResolver) parseRSAPrivateKey(derBytes []byte, alg string) (*rsa.PrivateKey, error) {
	privateKey, err := x509.ParsePKCS1PrivateKey(derBytes)
	if err != nil {
		return nil, errors.WithCode(
			code.ErrInvalidJWK,
			"failed to parse RSA private key: %v",
			err,
		)
	}

	// 验证密钥大小
	keySize := privateKey.N.BitLen()
	if err := ValidateKeySize(keySize); err != nil {
		return nil, errors.WithCode(
			code.ErrInvalidJWK,
			"invalid RSA key size: %v",
			err,
		)
	}

	// 验证算法匹配
	if !isRSAAlgorithm(alg) {
		return nil, errors.WithCode(
			code.ErrUnsupportedKty,
			"algorithm %s is not compatible with RSA private key",
			alg,
		)
	}

	return privateKey, nil
}

// parsePKCS8PrivateKey 解析 PKCS#8 格式的私钥
func (r *PEMPrivateKeyResolver) parsePKCS8PrivateKey(derBytes []byte, alg string) (any, error) {
	privateKey, err := x509.ParsePKCS8PrivateKey(derBytes)
	if err != nil {
		return nil, errors.WithCode(
			code.ErrInvalidJWK,
			"failed to parse PKCS8 private key: %v",
			err,
		)
	}

	// 检查私钥类型
	switch key := privateKey.(type) {
	case *rsa.PrivateKey:
		// 验证密钥大小
		keySize := key.N.BitLen()
		if err := ValidateKeySize(keySize); err != nil {
			return nil, errors.WithCode(
				code.ErrInvalidJWK,
				"invalid RSA key size: %v",
				err,
			)
		}

		// 验证算法匹配
		if !isRSAAlgorithm(alg) {
			return nil, errors.WithCode(
				code.ErrUnsupportedKty,
				"algorithm %s is not compatible with RSA private key",
				alg,
			)
		}

		return key, nil

	default:
		return nil, errors.WithCode(
			code.ErrUnsupportedKty,
			"unsupported private key type: %T (only RSA is currently supported)",
			privateKey,
		)
	}
}

// isRSAAlgorithm 检查是否是 RSA 算法
func isRSAAlgorithm(alg string) bool {
	switch alg {
	case "RS256", "RS384", "RS512":
		return true
	default:
		return false
	}
}

// GetKeysDir 获取密钥目录路径
func (r *PEMPrivateKeyResolver) GetKeysDir() string {
	return r.keysDir
}

// ListKeyFiles 列出所有可用的密钥文件
func (r *PEMPrivateKeyResolver) ListKeyFiles() ([]string, error) {
	entries, err := os.ReadDir(r.keysDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read keys directory: %w", err)
	}

	var keyFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".pem" {
			keyFiles = append(keyFiles, entry.Name())
		}
	}

	return keyFiles, nil
}
