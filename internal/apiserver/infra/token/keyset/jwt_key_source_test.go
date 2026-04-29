package keyset

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJWTKeySourceReturnsActiveSigningKeyAndVerificationKey(t *testing.T) {
	t.Parallel()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	kid := "jwt-key-source-test"
	key := NewKey(kid, publicJWKForTest(kid, &privateKey.PublicKey))

	source := NewJWTKeySource(
		jwtKeySourceManagerStub{active: key},
		jwtKeySourceResolverStub{privateKey: privateKey},
	)

	activeKid, signingKey, err := source.ActiveSigningKey(context.Background())
	require.NoError(t, err)
	require.Equal(t, kid, activeKid)
	require.Same(t, privateKey, signingKey)

	verificationKey, err := source.VerificationKey(context.Background(), kid)
	require.NoError(t, err)
	require.Equal(t, privateKey.PublicKey.N, verificationKey.N)
	require.Equal(t, privateKey.PublicKey.E, verificationKey.E)
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
