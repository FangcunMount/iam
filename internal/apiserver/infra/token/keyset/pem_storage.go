package keyset

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// PEMPrivateKeyStorage PEM 文件私钥存储
// 将私钥保存为 PEM 格式文件到指定目录
// 适用于开发环境和简单场景
// 生产环境建议使用 KMS/HSM
type PEMPrivateKeyStorage struct {
	// keysDir 私钥文件存储目录
	// 文件命名规则：{kid}.pem
	keysDir string

	// fileMode 文件权限（默认 0600，仅所有者可读写）
	fileMode os.FileMode
}

var _ PrivateKeyStorage = (*PEMPrivateKeyStorage)(nil)

// NewPEMPrivateKeyStorage 创建 PEM 私钥存储
func NewPEMPrivateKeyStorage(keysDir string) *PEMPrivateKeyStorage {
	return &PEMPrivateKeyStorage{
		keysDir:  keysDir,
		fileMode: 0600, // 仅所有者可读写，确保安全
	}
}

// NewPEMPrivateKeyStorageWithMode 创建指定权限的 PEM 私钥存储
func NewPEMPrivateKeyStorageWithMode(keysDir string, fileMode os.FileMode) *PEMPrivateKeyStorage {
	return &PEMPrivateKeyStorage{
		keysDir:  keysDir,
		fileMode: fileMode,
	}
}

// SavePrivateKey 保存私钥到 PEM 文件
func (s *PEMPrivateKeyStorage) SavePrivateKey(ctx context.Context, kid string, privateKey any, alg string) error {
	// 确保目录存在
	if err := s.ensureDir(); err != nil {
		return err
	}

	// 根据密钥类型编码为 PEM
	pemData, err := s.encodeToPEM(privateKey, alg)
	if err != nil {
		return err
	}

	// 构建文件路径
	filePath := s.getFilePath(kid)

	// 写入文件（使用安全的文件权限）
	if err := os.WriteFile(filePath, pemData, s.fileMode); err != nil {
		return errors.WithCode(
			code.ErrUnknown,
			"failed to write private key file %s: %v",
			filePath,
			err,
		)
	}

	return nil
}

// DeletePrivateKey 删除私钥文件
func (s *PEMPrivateKeyStorage) DeletePrivateKey(ctx context.Context, kid string) error {
	filePath := s.getFilePath(kid)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return errors.WithCode(
			code.ErrKeyNotFound,
			"private key file not found: %s",
			filePath,
		)
	}

	// 删除文件
	if err := os.Remove(filePath); err != nil {
		return errors.WithCode(
			code.ErrUnknown,
			"failed to delete private key file %s: %v",
			filePath,
			err,
		)
	}

	return nil
}

// KeyExists 检查私钥文件是否存在
func (s *PEMPrivateKeyStorage) KeyExists(ctx context.Context, kid string) (bool, error) {
	filePath := s.getFilePath(kid)
	_, err := os.Stat(filePath)

	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, errors.WithCode(
		code.ErrUnknown,
		"failed to check key existence: %v",
		err,
	)
}

// ensureDir 确保存储目录存在
func (s *PEMPrivateKeyStorage) ensureDir() error {
	if err := os.MkdirAll(s.keysDir, 0755); err != nil {
		return errors.WithCode(
			code.ErrUnknown,
			"failed to create keys directory %s: %v",
			s.keysDir,
			err,
		)
	}
	return nil
}

// getFilePath 获取密钥文件路径
func (s *PEMPrivateKeyStorage) getFilePath(kid string) string {
	return filepath.Join(s.keysDir, kid+".pem")
}

// encodeToPEM 将私钥编码为 PEM 格式
func (s *PEMPrivateKeyStorage) encodeToPEM(privateKey any, alg string) ([]byte, error) {
	switch alg {
	case "RS256", "RS384", "RS512":
		return s.encodeRSAPrivateKey(privateKey)
	// 未来可以支持 EC 和 EdDSA
	// case "ES256", "ES384", "ES512":
	//     return s.encodeECPrivateKey(privateKey)
	// case "EdDSA":
	//     return s.encodeEdDSAPrivateKey(privateKey)
	default:
		return nil, errors.WithCode(
			code.ErrUnsupportedKty,
			"unsupported algorithm for PEM encoding: %s",
			alg,
		)
	}
}

// encodeRSAPrivateKey 编码 RSA 私钥为 PKCS#8 PEM 格式
func (s *PEMPrivateKeyStorage) encodeRSAPrivateKey(privateKey any) ([]byte, error) {
	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.WithCode(
			code.ErrInvalidJWK,
			"expected *rsa.PrivateKey, got %T",
			privateKey,
		)
	}

	// 使用 PKCS#8 格式（更通用）
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	if err != nil {
		return nil, errors.WithCode(
			code.ErrUnknown,
			"failed to marshal RSA private key to PKCS#8: %v",
			err,
		)
	}

	// 编码为 PEM
	pemBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	}

	return pem.EncodeToMemory(pemBlock), nil
}

// GetKeysDir 获取密钥存储目录（用于测试）
func (s *PEMPrivateKeyStorage) GetKeysDir() string {
	return s.keysDir
}

// ListKeys 列出所有密钥 ID（用于调试/管理）
func (s *PEMPrivateKeyStorage) ListKeys(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(s.keysDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, errors.WithCode(
			code.ErrUnknown,
			"failed to read keys directory: %v",
			err,
		)
	}

	var kids []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".pem" {
			// 去掉 .pem 后缀得到 kid
			kid := entry.Name()[:len(entry.Name())-4]
			kids = append(kids, kid)
		}
	}

	return kids, nil
}
