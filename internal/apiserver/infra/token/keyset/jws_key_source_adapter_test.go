package keyset

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestJWSKeySourceAdapterReturnsActiveSigningKeyAndVerificationKey(t *testing.T) {
	t.Parallel()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	kid := "jwt-key-source-test"
	key := NewKey(kid, publicJWKForTest(kid, &privateKey.PublicKey))

	source := NewJWSKeySourceAdapter(
		jwtKeySourceManagerStub{active: key},
		jwtKeySourceResolverStub{privateKey: privateKey},
	)

	signingKey, err := source.ActiveSigningKey(context.Background())
	require.NoError(t, err)
	require.Equal(t, kid, signingKey.Kid)
	require.Equal(t, "RS256", signingKey.Algorithm)
	require.Same(t, privateKey, signingKey.PrivateKey)

	verificationKey, err := source.VerificationKey(context.Background(), kid)
	require.NoError(t, err)
	require.Equal(t, kid, verificationKey.Kid)
	require.Equal(t, "RS256", verificationKey.Algorithm)
	require.Equal(t, privateKey.PublicKey.N, verificationKey.PublicKey.N)
	require.Equal(t, privateKey.PublicKey.E, verificationKey.PublicKey.E)
}

func TestJWSKeySourceAdapterRejectsKeysOutsideVerificationLifecycle(t *testing.T) {
	t.Parallel()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now()

	tests := []struct {
		name string
		key  *Key
	}{
		{name: "retired", key: NewKey("retired", publicJWKForTest("retired", &privateKey.PublicKey), WithStatus(KeyRetired))},
		{name: "not yet valid", key: NewKey("future", publicJWKForTest("future", &privateKey.PublicKey), WithNotBefore(now.Add(time.Hour)))},
		{name: "expired", key: NewKey("expired", publicJWKForTest("expired", &privateKey.PublicKey), WithNotAfter(now.Add(-time.Hour)))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewJWSKeySourceAdapter(jwtKeySourceManagerStub{active: tt.key}, jwtKeySourceResolverStub{privateKey: privateKey})
			_, err := source.VerificationKey(context.Background(), tt.key.Kid)
			require.Error(t, err)
		})
	}
}

type jwtKeySourceManagerStub struct {
	active *Key
}

func (s jwtKeySourceManagerStub) GetActiveKey(context.Context) (*Key, error) {
	return s.active, nil
}

func (s jwtKeySourceManagerStub) GetKeyByKid(context.Context, string) (*Key, error) {
	return s.active, nil
}

type jwtKeySourceResolverStub struct {
	privateKey *rsa.PrivateKey
}

func (s jwtKeySourceResolverStub) ResolveSigningKey(context.Context, string, string) (any, error) {
	return s.privateKey, nil
}

func publicJWKForTest(kid string, publicKey *rsa.PublicKey) PublicJWK {
	n := base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes())
	return PublicJWK{
		Kty: "RSA",
		Use: "sig",
		Alg: "RS256",
		Kid: kid,
		N:   &n,
		E:   &e,
	}
}
