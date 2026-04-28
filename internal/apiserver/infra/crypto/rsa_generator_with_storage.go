package crypto

import (
	"context"

	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/jwks"
)

// RSAKeyGeneratorWithStorage RSA 密钥生成器（带私钥持久化）
// 生成密钥后自动将私钥保存到存储中
type RSAKeyGeneratorWithStorage struct {
	generator      *RSAKeyGenerator
	privateStorage jwks.PrivateKeyStorage
}

// NewRSAKeyGeneratorWithStorage 创建带存储的 RSA 密钥生成器
func NewRSAKeyGeneratorWithStorage(privateStorage jwks.PrivateKeyStorage) *RSAKeyGeneratorWithStorage {
	return &RSAKeyGeneratorWithStorage{
		generator:      NewRSAKeyGenerator(),
		privateStorage: privateStorage,
	}
}

// NewRSAKeyGeneratorWithStorageAndSize 创建带存储和指定密钥大小的 RSA 密钥生成器
func NewRSAKeyGeneratorWithStorageAndSize(privateStorage jwks.PrivateKeyStorage, keySize int) *RSAKeyGeneratorWithStorage {
	return &RSAKeyGeneratorWithStorage{
		generator:      NewRSAKeyGeneratorWithSize(keySize),
		privateStorage: privateStorage,
	}
}

var _ jwks.KeyGenerator = (*RSAKeyGeneratorWithStorage)(nil)

// GenerateKeyPair 生成 RSA 密钥对并持久化私钥
func (g *RSAKeyGeneratorWithStorage) GenerateKeyPair(ctx context.Context, algorithm, kid string) (*jwks.KeyPair, error) {
	// 1. 生成密钥对
	keyPair, err := g.generator.GenerateKeyPair(ctx, algorithm, kid)
	if err != nil {
		return nil, err
	}

	// 2. 立即持久化私钥
	if err := g.privateStorage.SavePrivateKey(ctx, kid, keyPair.PrivateKey, algorithm); err != nil {
		return nil, err
	}

	// 3. 返回密钥对（公钥部分会被 KeyManager 保存到数据库）
	return keyPair, nil
}

// SupportedAlgorithms 返回支持的算法列表
func (g *RSAKeyGeneratorWithStorage) SupportedAlgorithms() []string {
	return g.generator.SupportedAlgorithms()
}
