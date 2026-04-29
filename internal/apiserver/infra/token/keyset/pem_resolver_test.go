package crypto

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestKeysDir 创建测试密钥目录
func setupTestKeysDir(t *testing.T) string {
	tmpDir := t.TempDir()
	return tmpDir
}

// generateTestRSAKey 生成测试用 RSA 私钥
func generateTestRSAKey(t *testing.T, keySize int) *rsa.PrivateKey {
	privateKey, err := rsa.GenerateKey(rand.Reader, keySize)
	require.NoError(t, err)
	return privateKey
}

// savePrivateKeyPKCS1 保存 PKCS#1 格式的私钥
func savePrivateKeyPKCS1(t *testing.T, dir, filename string, privateKey *rsa.PrivateKey) {
	pemPath := filepath.Join(dir, filename)

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	pemFile, err := os.Create(pemPath)
	require.NoError(t, err)
	defer func() { _ = pemFile.Close() }()

	err = pem.Encode(pemFile, pemBlock)
	require.NoError(t, err)
}

// savePrivateKeyPKCS8 保存 PKCS#8 格式的私钥
func savePrivateKeyPKCS8(t *testing.T, dir, filename string, privateKey *rsa.PrivateKey) {
	pemPath := filepath.Join(dir, filename)

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)

	pemBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	pemFile, err := os.Create(pemPath)
	require.NoError(t, err)
	defer func() { _ = pemFile.Close() }()

	err = pem.Encode(pemFile, pemBlock)
	require.NoError(t, err)
}

func TestNewPEMPrivateKeyResolver(t *testing.T) {
	keysDir := setupTestKeysDir(t)
	resolver := NewPEMPrivateKeyResolver(keysDir)

	assert.NotNil(t, resolver)

	pemResolver, ok := resolver.(*PEMPrivateKeyResolver)
	assert.True(t, ok)
	assert.Equal(t, keysDir, pemResolver.GetKeysDir())
}

func TestPEMPrivateKeyResolver_ResolveSigningKey_PKCS1(t *testing.T) {
	keysDir := setupTestKeysDir(t)
	resolver := NewPEMPrivateKeyResolver(keysDir)
	ctx := context.Background()

	// 生成并保存测试密钥 (PKCS#1 格式)
	privateKey := generateTestRSAKey(t, 2048)
	savePrivateKeyPKCS1(t, keysDir, "test-kid-001.pem", privateKey)

	// 解析私钥
	resolvedKey, err := resolver.ResolveSigningKey(ctx, "test-kid-001", "RS256")
	require.NoError(t, err)
	assert.NotNil(t, resolvedKey)

	// 验证返回的是 RSA 私钥
	rsaKey, ok := resolvedKey.(*rsa.PrivateKey)
	assert.True(t, ok)
	assert.NotNil(t, rsaKey)

	// 验证密钥内容
	assert.Equal(t, privateKey.N, rsaKey.N)
	assert.Equal(t, privateKey.E, rsaKey.E)
}

func TestPEMPrivateKeyResolver_ResolveSigningKey_PKCS8(t *testing.T) {
	keysDir := setupTestKeysDir(t)
	resolver := NewPEMPrivateKeyResolver(keysDir)
	ctx := context.Background()

	// 生成并保存测试密钥 (PKCS#8 格式)
	privateKey := generateTestRSAKey(t, 2048)
	savePrivateKeyPKCS8(t, keysDir, "test-kid-002.pem", privateKey)

	// 解析私钥
	resolvedKey, err := resolver.ResolveSigningKey(ctx, "test-kid-002", "RS256")
	require.NoError(t, err)
	assert.NotNil(t, resolvedKey)

	// 验证返回的是 RSA 私钥
	rsaKey, ok := resolvedKey.(*rsa.PrivateKey)
	assert.True(t, ok)
	assert.NotNil(t, rsaKey)
}

func TestPEMPrivateKeyResolver_ResolveSigningKey_WithKeyPrefix(t *testing.T) {
	keysDir := setupTestKeysDir(t)
	resolver := NewPEMPrivateKeyResolver(keysDir)
	ctx := context.Background()

	// 生成并保存测试密钥（使用 key- 前缀）
	privateKey := generateTestRSAKey(t, 2048)
	savePrivateKeyPKCS1(t, keysDir, "key-test-kid-003.pem", privateKey)

	// 解析私钥（不带前缀的 kid）
	resolvedKey, err := resolver.ResolveSigningKey(ctx, "test-kid-003", "RS256")
	require.NoError(t, err)
	assert.NotNil(t, resolvedKey)

	rsaKey, ok := resolvedKey.(*rsa.PrivateKey)
	assert.True(t, ok)
	assert.NotNil(t, rsaKey)
}

func TestPEMPrivateKeyResolver_ResolveSigningKey_FileNotFound(t *testing.T) {
	keysDir := setupTestKeysDir(t)
	resolver := NewPEMPrivateKeyResolver(keysDir)
	ctx := context.Background()

	// 尝试解析不存在的密钥
	resolvedKey, err := resolver.ResolveSigningKey(ctx, "non-existent", "RS256")
	assert.Error(t, err)
	assert.Nil(t, resolvedKey)
	assert.Contains(t, err.Error(), "not found")
}

func TestPEMPrivateKeyResolver_ResolveSigningKey_InvalidPEM(t *testing.T) {
	keysDir := setupTestKeysDir(t)
	resolver := NewPEMPrivateKeyResolver(keysDir)
	ctx := context.Background()

	// 创建无效的 PEM 文件
	invalidPEMPath := filepath.Join(keysDir, "invalid.pem")
	err := os.WriteFile(invalidPEMPath, []byte("not a valid PEM file"), 0600)
	require.NoError(t, err)

	// 尝试解析
	resolvedKey, err := resolver.ResolveSigningKey(ctx, "invalid", "RS256")
	assert.Error(t, err)
	assert.Nil(t, resolvedKey)
	// PEM 解码失败会返回错误
}

func TestPEMPrivateKeyResolver_ResolveSigningKey_WrongAlgorithm(t *testing.T) {
	keysDir := setupTestKeysDir(t)
	resolver := NewPEMPrivateKeyResolver(keysDir)
	ctx := context.Background()

	// 生成并保存 RSA 密钥
	privateKey := generateTestRSAKey(t, 2048)
	savePrivateKeyPKCS1(t, keysDir, "rsa-key.pem", privateKey)

	// 尝试使用不兼容的算法（EC 算法）
	resolvedKey, err := resolver.ResolveSigningKey(ctx, "rsa-key", "ES256")
	assert.Error(t, err)
	assert.Nil(t, resolvedKey)
	// 算法不兼容会返回错误
}

func TestPEMPrivateKeyResolver_ResolveSigningKey_KeySizeTooSmall(t *testing.T) {
	keysDir := setupTestKeysDir(t)
	resolver := NewPEMPrivateKeyResolver(keysDir)
	ctx := context.Background()

	// 生成 1024 位密钥（太小）
	privateKey := generateTestRSAKey(t, 1024)
	savePrivateKeyPKCS1(t, keysDir, "small-key.pem", privateKey)

	// 尝试解析
	resolvedKey, err := resolver.ResolveSigningKey(ctx, "small-key", "RS256")
	assert.Error(t, err)
	assert.Nil(t, resolvedKey)
	// 密钥太小会返回错误
}

func TestPEMPrivateKeyResolver_ResolveSigningKey_DifferentAlgorithms(t *testing.T) {
	keysDir := setupTestKeysDir(t)
	resolver := NewPEMPrivateKeyResolver(keysDir)
	ctx := context.Background()

	// 生成并保存测试密钥
	privateKey := generateTestRSAKey(t, 2048)
	savePrivateKeyPKCS1(t, keysDir, "multi-alg.pem", privateKey)

	tests := []struct {
		name    string
		alg     string
		wantErr bool
	}{
		{
			name:    "RS256 成功",
			alg:     "RS256",
			wantErr: false,
		},
		{
			name:    "RS384 成功",
			alg:     "RS384",
			wantErr: false,
		},
		{
			name:    "RS512 成功",
			alg:     "RS512",
			wantErr: false,
		},
		{
			name:    "ES256 失败",
			alg:     "ES256",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolvedKey, err := resolver.ResolveSigningKey(ctx, "multi-alg", tt.alg)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resolvedKey)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resolvedKey)

				rsaKey, ok := resolvedKey.(*rsa.PrivateKey)
				assert.True(t, ok)
				assert.NotNil(t, rsaKey)
			}
		})
	}
}

func TestPEMPrivateKeyResolver_ListKeyFiles(t *testing.T) {
	keysDir := setupTestKeysDir(t)
	resolver := NewPEMPrivateKeyResolver(keysDir).(*PEMPrivateKeyResolver)

	// 创建多个密钥文件
	privateKey := generateTestRSAKey(t, 2048)
	savePrivateKeyPKCS1(t, keysDir, "key1.pem", privateKey)
	savePrivateKeyPKCS1(t, keysDir, "key2.pem", privateKey)
	savePrivateKeyPKCS1(t, keysDir, "key3.pem", privateKey)

	// 创建一个非 PEM 文件
	nonPEMPath := filepath.Join(keysDir, "not-a-key.txt")
	err := os.WriteFile(nonPEMPath, []byte("not a pem"), 0600)
	require.NoError(t, err)

	// 列出密钥文件
	keyFiles, err := resolver.ListKeyFiles()
	require.NoError(t, err)

	// 应该只返回 .pem 文件
	assert.Len(t, keyFiles, 3)
	assert.Contains(t, keyFiles, "key1.pem")
	assert.Contains(t, keyFiles, "key2.pem")
	assert.Contains(t, keyFiles, "key3.pem")
	assert.NotContains(t, keyFiles, "not-a-key.txt")
}

func TestPEMPrivateKeyResolver_GetPEMPath(t *testing.T) {
	keysDir := setupTestKeysDir(t)
	resolver := NewPEMPrivateKeyResolver(keysDir).(*PEMPrivateKeyResolver)

	tests := []struct {
		name          string
		kid           string
		existingFiles []string
		expectedFile  string
	}{
		{
			name:          "优先使用 {kid}.pem",
			kid:           "test-001",
			existingFiles: []string{"test-001.pem"},
			expectedFile:  "test-001.pem",
		},
		{
			name:          "回退到 key-{kid}.pem",
			kid:           "test-002",
			existingFiles: []string{"key-test-002.pem"},
			expectedFile:  "key-test-002.pem",
		},
		{
			name:          "两者都存在时优先 {kid}.pem",
			kid:           "test-003",
			existingFiles: []string{"test-003.pem", "key-test-003.pem"},
			expectedFile:  "test-003.pem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 清理目录
			entries, _ := os.ReadDir(keysDir)
			for _, entry := range entries {
				_ = os.Remove(filepath.Join(keysDir, entry.Name()))
			}

			// 创建测试文件
			for _, filename := range tt.existingFiles {
				path := filepath.Join(keysDir, filename)
				err := os.WriteFile(path, []byte("dummy"), 0600)
				require.NoError(t, err)
			}

			// 测试路径获取
			pemPath := resolver.getPEMPath(tt.kid)
			assert.Equal(t, filepath.Join(keysDir, tt.expectedFile), pemPath)
		})
	}
}

// Benchmark 测试
func BenchmarkPEMPrivateKeyResolver_ResolveSigningKey(b *testing.B) {
	tmpDir := b.TempDir()
	resolver := NewPEMPrivateKeyResolver(tmpDir)
	ctx := context.Background()

	// 生成并保存测试密钥
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pemPath := filepath.Join(tmpDir, "bench-key.pem")

	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	pemFile, _ := os.Create(pemPath)
	_ = pem.Encode(pemFile, pemBlock)
	_ = pemFile.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resolver.ResolveSigningKey(ctx, "bench-key", "RS256")
		if err != nil {
			b.Fatal(err)
		}
	}
}
