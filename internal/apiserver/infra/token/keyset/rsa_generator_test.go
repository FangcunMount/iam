package crypto

import (
	"context"
	"crypto/rsa"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRSAKeyGenerator(t *testing.T) {
	gen := NewRSAKeyGenerator()
	assert.NotNil(t, gen)
	assert.Equal(t, 2048, gen.GetKeySize())
}

func TestNewRSAKeyGeneratorWithSize(t *testing.T) {
	tests := []struct {
		name    string
		keySize int
	}{
		{"2048 bits", 2048},
		{"4096 bits", 4096},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewRSAKeyGeneratorWithSize(tt.keySize)
			assert.NotNil(t, gen)
			assert.Equal(t, tt.keySize, gen.GetKeySize())
		})
	}
}

func TestRSAKeyGenerator_SupportedAlgorithms(t *testing.T) {
	gen := NewRSAKeyGenerator()
	algs := gen.SupportedAlgorithms()

	assert.Len(t, algs, 3)
	assert.Contains(t, algs, "RS256")
	assert.Contains(t, algs, "RS384")
	assert.Contains(t, algs, "RS512")
}

func TestRSAKeyGenerator_GenerateKeyPair(t *testing.T) {
	gen := NewRSAKeyGenerator()
	ctx := context.Background()

	tests := []struct {
		name    string
		alg     string
		kid     string
		wantErr bool
	}{
		{
			name:    "RS256 成功",
			alg:     "RS256",
			kid:     "test-kid-256",
			wantErr: false,
		},
		{
			name:    "RS384 成功",
			alg:     "RS384",
			kid:     "test-kid-384",
			wantErr: false,
		},
		{
			name:    "RS512 成功",
			alg:     "RS512",
			kid:     "test-kid-512",
			wantErr: false,
		},
		{
			name:    "不支持的算法",
			alg:     "RS128",
			kid:     "test-kid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyPair, err := gen.GenerateKeyPair(ctx, tt.alg, tt.kid)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, keyPair)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, keyPair)

			// 验证私钥
			assert.NotNil(t, keyPair.PrivateKey)
			privateKey, ok := keyPair.PrivateKey.(*rsa.PrivateKey)
			assert.True(t, ok, "PrivateKey should be *rsa.PrivateKey")
			assert.NotNil(t, privateKey)

			// 验证公钥 JWK
			jwk := keyPair.PublicJWK
			assert.Equal(t, "RSA", jwk.Kty)
			assert.Equal(t, "sig", jwk.Use)
			assert.Equal(t, tt.alg, jwk.Alg)
			assert.Equal(t, tt.kid, jwk.Kid)
			assert.NotNil(t, jwk.N)
			assert.NotNil(t, jwk.E)
			assert.NotEmpty(t, *jwk.N)
			assert.NotEmpty(t, *jwk.E)

			// 验证 JWK 有效性
			err = jwk.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestRSAKeyGenerator_GenerateKeyPair_DifferentKeySizes(t *testing.T) {
	ctx := context.Background()
	alg := "RS256"
	kid := "test-kid"

	tests := []struct {
		name    string
		keySize int
	}{
		{"2048 bits", 2048},
		{"4096 bits", 4096},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewRSAKeyGeneratorWithSize(tt.keySize)
			keyPair, err := gen.GenerateKeyPair(ctx, alg, kid)

			require.NoError(t, err)
			require.NotNil(t, keyPair)

			privateKey := keyPair.PrivateKey.(*rsa.PrivateKey)
			// 验证密钥大小
			actualSize := privateKey.N.BitLen()
			// 允许少量误差（通常是 keySize-1 到 keySize）
			assert.True(t, actualSize >= tt.keySize-1 && actualSize <= tt.keySize,
				"key size should be around %d, got %d", tt.keySize, actualSize)
		})
	}
}

func TestValidateKeySize(t *testing.T) {
	tests := []struct {
		name    string
		keySize int
		wantErr bool
	}{
		{"2048 bits - 有效", 2048, false},
		{"4096 bits - 有效", 4096, false},
		{"8192 bits - 有效", 8192, false},
		{"1024 bits - 太小", 1024, true},
		{"2000 bits - 非1024倍数", 2000, true},
		{"10240 bits - 太大", 10240, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKeySize(tt.keySize)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetAlgorithmInfo(t *testing.T) {
	tests := []struct {
		name        string
		alg         string
		wantNil     bool
		wantHash    string
		wantRecSize int
		wantMinSize int
		wantDesc    string
	}{
		{
			name:        "RS256",
			alg:         "RS256",
			wantNil:     false,
			wantHash:    "SHA-256",
			wantRecSize: 2048,
			wantMinSize: 2048,
			wantDesc:    "RSA Signature with SHA-256",
		},
		{
			name:        "RS384",
			alg:         "RS384",
			wantNil:     false,
			wantHash:    "SHA-384",
			wantRecSize: 2048,
			wantMinSize: 2048,
			wantDesc:    "RSA Signature with SHA-384",
		},
		{
			name:        "RS512",
			alg:         "RS512",
			wantNil:     false,
			wantHash:    "SHA-512",
			wantRecSize: 4096,
			wantMinSize: 2048,
			wantDesc:    "RSA Signature with SHA-512",
		},
		{
			name:    "Unknown algorithm",
			alg:     "RS128",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := GetAlgorithmInfo(tt.alg)

			if tt.wantNil {
				assert.Nil(t, info)
				return
			}

			require.NotNil(t, info)
			assert.Equal(t, tt.alg, info.Algorithm)
			assert.Equal(t, tt.wantHash, info.HashAlgorithm)
			assert.Equal(t, tt.wantRecSize, info.RecommendedSize)
			assert.Equal(t, tt.wantMinSize, info.MinimumSize)
			assert.Equal(t, tt.wantDesc, info.Description)
		})
	}
}

// Benchmark 测试
func BenchmarkRSAKeyGenerator_GenerateKeyPair_2048(b *testing.B) {
	gen := NewRSAKeyGeneratorWithSize(2048)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gen.GenerateKeyPair(ctx, "RS256", "bench-kid")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRSAKeyGenerator_GenerateKeyPair_4096(b *testing.B) {
	gen := NewRSAKeyGeneratorWithSize(4096)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gen.GenerateKeyPair(ctx, "RS512", "bench-kid")
		if err != nil {
			b.Fatal(err)
		}
	}
}
