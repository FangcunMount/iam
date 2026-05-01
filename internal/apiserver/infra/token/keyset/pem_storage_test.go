package keyset_test

import (
	"context"
	"crypto/rsa"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/v2/internal/apiserver/infra/token/keyset"
)

func TestPEMPrivateKeyStorage_SaveAndExists(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()

	// 创建存储
	storage := keyset.NewPEMPrivateKeyStorage(tempDir)

	ctx := context.Background()
	kid := "test-kid-001"
	alg := "RS256"

	// 生成测试密钥
	generator := keyset.NewRSAKeyGenerator()
	keyPair, err := generator.GenerateKeyPair(ctx, alg, kid)
	require.NoError(t, err)

	// 保存私钥
	err = storage.SavePrivateKey(ctx, kid, keyPair.PrivateKey, alg)
	require.NoError(t, err)

	// 验证文件存在
	expectedPath := filepath.Join(tempDir, kid+".pem")
	_, err = os.Stat(expectedPath)
	assert.NoError(t, err, "PEM file should exist")

	// 验证 KeyExists 返回 true
	exists, err := storage.KeyExists(ctx, kid)
	require.NoError(t, err)
	assert.True(t, exists)

	// 验证文件权限（应该是 0600）
	info, err := os.Stat(expectedPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	t.Logf("✅ Private key saved to: %s", expectedPath)
}

func TestPEMPrivateKeyStorage_DeleteKey(t *testing.T) {
	tempDir := t.TempDir()
	storage := keyset.NewPEMPrivateKeyStorage(tempDir)
	ctx := context.Background()
	kid := "test-kid-002"
	alg := "RS256"

	// 生成并保存密钥
	generator := keyset.NewRSAKeyGenerator()
	keyPair, err := generator.GenerateKeyPair(ctx, alg, kid)
	require.NoError(t, err)

	err = storage.SavePrivateKey(ctx, kid, keyPair.PrivateKey, alg)
	require.NoError(t, err)

	// 验证存在
	exists, err := storage.KeyExists(ctx, kid)
	require.NoError(t, err)
	assert.True(t, exists)

	// 删除密钥
	err = storage.DeletePrivateKey(ctx, kid)
	require.NoError(t, err)

	// 验证不存在
	exists, err = storage.KeyExists(ctx, kid)
	require.NoError(t, err)
	assert.False(t, exists)

	t.Log("✅ Private key deleted successfully")
}

func TestPEMPrivateKeyStorage_ListKeys(t *testing.T) {
	tempDir := t.TempDir()
	storage := keyset.NewPEMPrivateKeyStorage(tempDir)
	ctx := context.Background()
	alg := "RS256"

	// 生成多个密钥
	generator := keyset.NewRSAKeyGenerator()
	kids := []string{"key-001", "key-002", "key-003"}

	for _, kid := range kids {
		keyPair, err := generator.GenerateKeyPair(ctx, alg, kid)
		require.NoError(t, err)

		err = storage.SavePrivateKey(ctx, kid, keyPair.PrivateKey, alg)
		require.NoError(t, err)
	}

	// 列出所有密钥
	listedKids, err := storage.ListKeys(ctx)
	require.NoError(t, err)

	assert.Equal(t, len(kids), len(listedKids))
	assert.ElementsMatch(t, kids, listedKids)

	t.Logf("✅ Listed %d keys: %v", len(listedKids), listedKids)
}

func TestRSAKeyGeneratorWithStorage_Integration(t *testing.T) {
	tempDir := t.TempDir()
	storage := keyset.NewPEMPrivateKeyStorage(tempDir)
	generator := keyset.NewRSAKeyGeneratorWithStorage(storage)

	ctx := context.Background()
	kid := "test-integration-key"
	alg := "RS256"

	// 生成密钥（会自动保存私钥）
	keyPair, err := generator.GenerateKeyPair(ctx, alg, kid)
	require.NoError(t, err)
	require.NotNil(t, keyPair)

	// 验证返回的密钥对
	assert.NotNil(t, keyPair.PrivateKey)
	assert.NotNil(t, keyPair.PublicJWK)
	assert.Equal(t, kid, keyPair.PublicJWK.Kid)
	assert.Equal(t, "RSA", keyPair.PublicJWK.Kty)
	assert.Equal(t, alg, keyPair.PublicJWK.Alg)

	// 验证私钥已保存
	exists, err := storage.KeyExists(ctx, kid)
	require.NoError(t, err)
	assert.True(t, exists, "Private key should be saved automatically")

	// 验证可以读取私钥
	resolver := keyset.NewPEMPrivateKeyResolver(tempDir)
	resolvedKey, err := resolver.ResolveSigningKey(ctx, kid, alg)
	require.NoError(t, err)
	require.NotNil(t, resolvedKey)

	// 验证解析出的密钥类型正确
	rsaKey, ok := resolvedKey.(*rsa.PrivateKey)
	require.True(t, ok, "Resolved key should be *rsa.PrivateKey")
	assert.NotNil(t, rsaKey)

	t.Log("✅ Key generation with automatic storage works correctly")
}

func TestRSAKeyGeneratorWithStorage_DifferentAlgorithms(t *testing.T) {
	tempDir := t.TempDir()
	storage := keyset.NewPEMPrivateKeyStorage(tempDir)
	generator := keyset.NewRSAKeyGeneratorWithStorage(storage)

	ctx := context.Background()
	algorithms := []string{"RS256", "RS384", "RS512"}

	for _, alg := range algorithms {
		kid := "test-key-" + alg
		keyPair, err := generator.GenerateKeyPair(ctx, alg, kid)
		require.NoError(t, err, "Failed to generate key for %s", alg)
		assert.Equal(t, alg, keyPair.PublicJWK.Alg)

		// 验证私钥已保存
		exists, err := storage.KeyExists(ctx, kid)
		require.NoError(t, err)
		assert.True(t, exists, "Private key for %s should be saved", alg)

		t.Logf("✅ Generated and saved key for algorithm: %s", alg)
	}
}

func TestPEMPrivateKeyStorage_CompatibilityWithResolver(t *testing.T) {
	// 测试存储和解析器的兼容性
	tempDir := t.TempDir()
	storage := keyset.NewPEMPrivateKeyStorage(tempDir)
	resolver := keyset.NewPEMPrivateKeyResolver(tempDir)

	ctx := context.Background()
	kid := "compatibility-test-key"
	alg := "RS256"

	// 生成密钥
	generator := keyset.NewRSAKeyGenerator()
	keyPair, err := generator.GenerateKeyPair(ctx, alg, kid)
	require.NoError(t, err)

	originalKey := keyPair.PrivateKey.(*rsa.PrivateKey)

	// 使用存储保存
	err = storage.SavePrivateKey(ctx, kid, keyPair.PrivateKey, alg)
	require.NoError(t, err)

	// 使用解析器读取
	resolvedKey, err := resolver.ResolveSigningKey(ctx, kid, alg)
	require.NoError(t, err)

	resolvedRSAKey := resolvedKey.(*rsa.PrivateKey)

	// 验证密钥内容一致
	assert.Equal(t, originalKey.N, resolvedRSAKey.N, "Modulus should match")
	assert.Equal(t, originalKey.E, resolvedRSAKey.E, "Exponent should match")
	assert.Equal(t, originalKey.D, resolvedRSAKey.D, "Private exponent should match")

	t.Log("✅ Storage and Resolver are compatible")
}
