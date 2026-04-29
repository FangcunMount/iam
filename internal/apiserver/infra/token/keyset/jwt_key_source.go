package keyset

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
)

type JWTKeySource struct {
	manager interface {
		GetActiveKey(ctx context.Context) (*Key, error)
		GetKeyByKid(ctx context.Context, kid string) (*Key, error)
	}
	keyResolver PrivateKeyResolver
}

func NewJWTKeySource(manager interface {
	GetActiveKey(ctx context.Context) (*Key, error)
	GetKeyByKid(ctx context.Context, kid string) (*Key, error)
}, keyResolver PrivateKeyResolver) *JWTKeySource {
	return &JWTKeySource{
		manager:     manager,
		keyResolver: keyResolver,
	}
}

func (s *JWTKeySource) ActiveSigningKey(ctx context.Context) (string, *rsa.PrivateKey, error) {
	activeKey, err := s.manager.GetActiveKey(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get active key: %w", err)
	}
	if activeKey == nil {
		return "", nil, fmt.Errorf("active key is nil")
	}
	rawKey, err := s.keyResolver.ResolveSigningKey(ctx, activeKey.Kid, activeKey.JWK.Alg)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve private key: %w", err)
	}
	privateKey, ok := rawKey.(*rsa.PrivateKey)
	if !ok {
		return "", nil, fmt.Errorf("expected RSA private key, got %T", rawKey)
	}
	return activeKey.Kid, privateKey, nil
}

func (s *JWTKeySource) VerificationKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	key, err := s.manager.GetKeyByKid(ctx, kid)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, fmt.Errorf("key not found for kid %s", kid)
	}
	return publicRSAKeyFromJWK(key.JWK)
}

func publicRSAKeyFromJWK(jwk PublicJWK) (*rsa.PublicKey, error) {
	if jwk.Kty != "RSA" {
		return nil, fmt.Errorf("unsupported key kty for verification: %s", jwk.Kty)
	}
	if jwk.N == nil || jwk.E == nil {
		return nil, fmt.Errorf("missing RSA parameters in JWK for kid %s", jwk.Kid)
	}
	nBytes, err := base64.RawURLEncoding.DecodeString(*jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to base64url-decode n for kid %s: %w", jwk.Kid, err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(*jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to base64url-decode e for kid %s: %w", jwk.Kid, err)
	}
	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	if e == 0 {
		return nil, fmt.Errorf("invalid exponent parsed for kid %s", jwk.Kid)
	}
	return &rsa.PublicKey{N: n, E: e}, nil
}
