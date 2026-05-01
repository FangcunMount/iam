package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
)

// secretVault 密钥加密服务实现（AES-GCM）
type secretVault struct {
	masterKey []byte // 主密钥（32 字节，用于 AES-256）
	gcm       cipher.AEAD
}

// 确保实现了接口
var _ wechatapp.SecretVault = (*secretVault)(nil)

// NewSecretVault 创建密钥加密服务实例
func NewSecretVault(masterKey []byte) (wechatapp.SecretVault, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("master key must be 32 bytes for AES-256")
	}

	// 创建 AES cipher
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// 创建 GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &secretVault{
		masterKey: masterKey,
		gcm:       gcm,
	}, nil
}

// Encrypt 加密明文
func (v *secretVault) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, errors.New("plaintext cannot be empty")
	}

	// 生成随机 nonce
	nonce := make([]byte, v.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// 加密
	// 格式: nonce || ciphertext
	ciphertext := v.gcm.Seal(nonce, nonce, plaintext, nil)

	return ciphertext, nil
}

// Decrypt 解密密文
func (v *secretVault) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, errors.New("ciphertext cannot be empty")
	}

	nonceSize := v.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	// 提取 nonce 和密文
	nonce, encryptedData := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// 解密
	plaintext, err := v.gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// Sign 签名（暂不实现，返回错误）
func (v *secretVault) Sign(ctx context.Context, keyRef string, data []byte) ([]byte, error) {
	return nil, errors.New("sign not implemented in local vault, use KMS for signing")
}
