package verifier

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	authnv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/authn/v2"
	authjwks "github.com/FangcunMount/iam/v3/pkg/sdk/auth/jwks"
	"github.com/FangcunMount/iam/v3/pkg/sdk/config"
	iamerrors "github.com/FangcunMount/iam/v3/pkg/sdk/errors"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type verifyStrategyStub struct {
	name      string
	result    *VerifyResult
	err       error
	callCount int
	lastToken string
	lastOpts  *VerifyOptions
}

func (s *verifyStrategyStub) Verify(_ context.Context, token string, opts *VerifyOptions) (*VerifyResult, error) {
	s.callCount++
	s.lastToken = token
	s.lastOpts = opts
	return s.result, s.err
}

func (s *verifyStrategyStub) Name() string {
	if s.name != "" {
		return s.name
	}
	return "stub"
}

type verifyTokenClientStub struct {
	verifyReq  *authnv2.VerifyTokenRequest
	verifyResp *authnv2.VerifyTokenResponse
	verifyErr  error
	callCount  int
}

type staticKeyFetcher struct {
	keySet jwk.Set
}

func (f *staticKeyFetcher) Fetch(context.Context) (jwk.Set, error) {
	return f.keySet, nil
}

func (f *staticKeyFetcher) Name() string { return "static" }

func newRS256Fixture(t *testing.T) (*rsa.PrivateKey, *authjwks.JWKSManager) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicJWK, err := jwk.FromRaw(&privateKey.PublicKey)
	require.NoError(t, err)
	require.NoError(t, publicJWK.Set(jwk.KeyIDKey, "kid-rs256"))
	require.NoError(t, publicJWK.Set(jwk.AlgorithmKey, jwa.RS256))

	keySet := jwk.NewSet()
	require.NoError(t, keySet.AddKey(publicJWK))
	manager, err := authjwks.NewJWKSManager(
		&config.JWKSConfig{URL: "https://unused.invalid/.well-known/jwks.json"},
		authjwks.WithCustomChain(&staticKeyFetcher{keySet: keySet}),
		authjwks.WithCacheEnabled(false),
	)
	require.NoError(t, err)
	t.Cleanup(manager.Stop)
	return privateKey, manager
}

func signRS256Token(t *testing.T, privateKey *rsa.PrivateKey, claims map[string]interface{}) string {
	t.Helper()

	token := jwt.New()
	for name, value := range claims {
		require.NoError(t, token.Set(name, value))
	}
	headers := jws.NewHeaders()
	require.NoError(t, headers.Set(jwk.KeyIDKey, "kid-rs256"))
	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, privateKey, jws.WithProtectedHeaders(headers)))
	require.NoError(t, err)
	return string(signed)
}

func (s *verifyTokenClientStub) VerifyToken(_ context.Context, in *authnv2.VerifyTokenRequest) (*authnv2.VerifyTokenResponse, error) {
	s.callCount++
	s.verifyReq = in
	return s.verifyResp, s.verifyErr
}

func TestRemoteVerifyStrategyPassesConfiguredIssuerAndAudience(t *testing.T) {
	privateKey, _ := newRS256Fixture(t)
	token := signRS256Token(t, privateKey, map[string]interface{}{
		jwt.SubjectKey:    "user:1",
		jwt.IssuerKey:     "https://iam.fangcunmount.cn",
		jwt.AudienceKey:   []string{"qs-api", "collection-api"},
		jwt.ExpirationKey: time.Now().Add(time.Minute),
	})
	stub := &verifyTokenClientStub{
		verifyResp: &authnv2.VerifyTokenResponse{
			Valid: true,
			Claims: &authnv2.TokenClaims{
				TokenId:         "jti-1",
				Subject:         "user:1",
				SessionId:       "sid-1",
				UserId:          "1",
				LoginIdentityId: "2",
				TenantId:        "3",
				Issuer:          "https://iam.fangcunmount.cn",
				Audience:        []string{"qs-api", "collection-api"},
				Amr:             []string{"pwd"},
				IssuedAt:        timestamppb.New(time.Now()),
				ExpiresAt:       timestamppb.New(time.Now().Add(time.Minute)),
			},
		},
	}

	strategy := NewRemoteVerifyStrategy(stub, &config.TokenVerifyConfig{
		AllowedIssuer:   "https://iam.fangcunmount.cn",
		AllowedAudience: []string{"qs-api"},
	})

	_, err := strategy.Verify(context.Background(), token, nil)
	require.NoError(t, err)
	require.NotNil(t, stub.verifyReq)
	require.Equal(t, "https://iam.fangcunmount.cn", stub.verifyReq.ExpectedIssuer)
	require.Equal(t, []string{"qs-api"}, stub.verifyReq.ExpectedAudience)
	require.Equal(t, []authnv2.TokenType{authnv2.TokenType_TOKEN_TYPE_ACCESS}, stub.verifyReq.AcceptedTokenTypes)
}

func TestRemoteVerifyStrategyOptionsOverrideConfig(t *testing.T) {
	privateKey, _ := newRS256Fixture(t)
	token := signRS256Token(t, privateKey, map[string]interface{}{
		jwt.SubjectKey:    "user:1",
		jwt.IssuerKey:     "https://issuer.override",
		jwt.AudienceKey:   []string{"collection-api"},
		jwt.ExpirationKey: time.Now().Add(time.Minute),
	})
	stub := &verifyTokenClientStub{
		verifyResp: &authnv2.VerifyTokenResponse{
			Valid: true,
			Claims: &authnv2.TokenClaims{
				TokenId:         "jti-2",
				Subject:         "user:1",
				SessionId:       "sid-override",
				UserId:          "1",
				LoginIdentityId: "2",
				TenantId:        "3",
				Issuer:          "https://issuer.override",
				Audience:        []string{"collection-api"},
				Amr:             []string{"pwd"},
				IssuedAt:        timestamppb.New(time.Now()),
				ExpiresAt:       timestamppb.New(time.Now().Add(time.Minute)),
			},
		},
	}

	strategy := NewRemoteVerifyStrategy(stub, &config.TokenVerifyConfig{
		AllowedIssuer:   "https://iam.fangcunmount.cn",
		AllowedAudience: []string{"qs-api"},
	})

	_, err := strategy.Verify(context.Background(), token, &VerifyOptions{
		ForceRemote:      true,
		IncludeMetadata:  true,
		ExpectedIssuer:   "https://issuer.override",
		ExpectedAudience: []string{"collection-api"},
	})
	require.NoError(t, err)
	require.NotNil(t, stub.verifyReq)
	require.Equal(t, "https://issuer.override", stub.verifyReq.ExpectedIssuer)
	require.Equal(t, []string{"collection-api"}, stub.verifyReq.ExpectedAudience)
	require.True(t, stub.verifyReq.ForceRemote)
	require.True(t, stub.verifyReq.IncludeMetadata)
}

func TestLocalVerifyStrategyRejectsServiceTokenByDefault(t *testing.T) {
	privateKey, manager := newRS256Fixture(t)
	token := signRS256Token(t, privateKey, map[string]interface{}{
		jwt.SubjectKey: "service:worker", jwt.ExpirationKey: time.Now().Add(time.Minute), "token_type": "service",
	})
	strategy := NewLocalVerifyStrategy(manager)

	result, err := strategy.Verify(context.Background(), token, nil)
	require.Error(t, err)
	require.ErrorIs(t, err, iamerrors.ErrTokenInvalid)
	require.Nil(t, result)

	result, err = strategy.Verify(context.Background(), token, &VerifyOptions{
		AllowedTokenTypes: []authnv2.TokenType{authnv2.TokenType_TOKEN_TYPE_SERVICE},
	})
	require.NoError(t, err)
	require.Equal(t, "service", result.Claims.TokenType)
}

func TestRemoteVerifyStrategyReturnsSessionID(t *testing.T) {
	privateKey, _ := newRS256Fixture(t)
	token := signRS256Token(t, privateKey, map[string]interface{}{
		jwt.SubjectKey:    "user:1",
		jwt.ExpirationKey: time.Now().Add(time.Minute),
	})
	stub := &verifyTokenClientStub{
		verifyResp: &authnv2.VerifyTokenResponse{
			Valid: true,
			Claims: &authnv2.TokenClaims{
				TokenId:         "jti-remote",
				Subject:         "user:1",
				SessionId:       "sid-remote",
				UserId:          "1",
				LoginIdentityId: "2",
				TenantId:        "fangcun",
				TenantDomain:    "fangcun",
				OrgId:           "42",
				TokenType:       authnv2.TokenType_TOKEN_TYPE_ACCESS,
				Issuer:          "https://iam.fangcunmount.cn",
				Audience:        []string{"qs-api"},
				Amr:             []string{"pwd", "otp"},
				IssuedAt:        timestamppb.New(time.Now()),
				NotBefore:       timestamppb.New(time.Now()),
				AuthenticatedAt: timestamppb.New(time.Now().Add(-time.Minute)),
				ExpiresAt:       timestamppb.New(time.Now().Add(time.Minute)),
				Attributes:      map[string]string{"auth_time": time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)},
			},
			Metadata: &authnv2.TokenMetadata{
				TokenType: authnv2.TokenType_TOKEN_TYPE_ACCESS,
				Status:    authnv2.TokenStatus_TOKEN_STATUS_VALID,
				IssuedAt:  timestamppb.New(time.Now()),
				ExpiresAt: timestamppb.New(time.Now().Add(time.Minute)),
			},
		},
	}

	strategy := NewRemoteVerifyStrategy(stub, &config.TokenVerifyConfig{})

	result, err := strategy.Verify(context.Background(), token, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Claims)
	require.Equal(t, "jti-remote", result.Claims.TokenID)
	require.Equal(t, "sid-remote", result.Claims.SessionID)
	require.Equal(t, "fangcun", result.Claims.TenantDomain)
	require.Equal(t, "42", result.Claims.OrgID)
	require.Equal(t, []string{"pwd", "otp"}, result.Claims.AMR)
	require.Equal(t, "access", result.Claims.TokenType)
	require.NotZero(t, result.Claims.NotBefore)
	require.NotZero(t, result.Claims.AuthenticatedAt)
	require.Equal(t, result.Claims.AuthenticatedAt, result.Claims.AuthTime)
	require.Contains(t, result.Claims.Attributes, "auth_time")
	require.NotNil(t, result.Metadata)
	require.Equal(t, authnv2.TokenType_TOKEN_TYPE_ACCESS, result.Metadata.TokenType)
	require.Equal(t, authnv2.TokenStatus_TOKEN_STATUS_VALID, result.Metadata.Status)
}

func TestExtractClaimsIncludesSessionID(t *testing.T) {
	token := jwt.New()
	require.NoError(t, token.Set(jwt.JwtIDKey, "jti-local"))
	require.NoError(t, token.Set(jwt.SubjectKey, "user:1"))
	require.NoError(t, token.Set("sid", "sid-local"))
	require.NoError(t, token.Set("user_id", "1"))
	require.NoError(t, token.Set("login_identity_id", "2"))
	require.NoError(t, token.Set("tenant_id", "fangcun"))
	require.NoError(t, token.Set("org_id", "3"))

	claims := extractClaims(token)
	require.NotNil(t, claims)
	require.Equal(t, "jti-local", claims.TokenID)
	require.Equal(t, "sid-local", claims.SessionID)
	require.Equal(t, "1", claims.UserID)
	require.Equal(t, "2", claims.LoginIdentityID)
	require.Equal(t, "fangcun", claims.TenantDomain)
	require.Equal(t, "3", claims.OrgID)
}

func TestBuildVerifyMetadataFromClaimsDefaultsAccessToken(t *testing.T) {
	metadata := buildVerifyMetadataFromClaims(&TokenClaims{
		TokenType: "",
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Minute),
	})
	require.NotNil(t, metadata)
	require.Equal(t, authnv2.TokenType_TOKEN_TYPE_ACCESS, metadata.TokenType)
	require.Equal(t, authnv2.TokenStatus_TOKEN_STATUS_VALID, metadata.Status)
}

func TestTokenVerifierForceRemoteUsesRemoteStrategy(t *testing.T) {
	local := &verifyStrategyStub{
		name: "local",
		result: &VerifyResult{
			Valid: true,
			Claims: &TokenClaims{
				Subject: "local-subject",
			},
		},
	}
	remote := &verifyStrategyStub{
		name: "remote",
		result: &VerifyResult{
			Valid: true,
			Claims: &TokenClaims{
				Subject: "remote-subject",
			},
		},
	}

	verifier := &TokenVerifier{
		strategy:       local,
		remoteStrategy: remote,
	}

	result, err := verifier.Verify(context.Background(), "jwt-token", &VerifyOptions{
		ForceRemote: true,
	})
	require.NoError(t, err)
	require.Equal(t, "remote-subject", result.Claims.Subject)
	require.Equal(t, 0, local.callCount)
	require.Equal(t, 1, remote.callCount)
}

func TestTokenVerifierForceRemoteWithoutRemoteStrategyFails(t *testing.T) {
	verifier := &TokenVerifier{
		strategy: &verifyStrategyStub{name: "local"},
	}

	result, err := verifier.Verify(context.Background(), "jwt-token", &VerifyOptions{
		ForceRemote: true,
	})
	require.Error(t, err)
	require.Nil(t, result)
}

func TestLocalVerifyStrategyAcceptsSingleAllowedAlgorithm(t *testing.T) {
	privateKey, manager := newRS256Fixture(t)
	strategy := NewLocalVerifyStrategy(manager, WithLocalConfig(&config.TokenVerifyConfig{
		AllowedIssuer:   "https://iam.fangcunmount.cn",
		AllowedAudience: []string{"qs-api"},
		RequiredClaims:  []string{"sub", "exp", "user_id", "tenant_id"},
		Algorithms:      []string{"RS256"},
	}))
	token := signRS256Token(t, privateKey, map[string]interface{}{
		jwt.SubjectKey:    "user:1",
		jwt.ExpirationKey: time.Now().Add(time.Minute),
		jwt.IssuerKey:     "https://iam.fangcunmount.cn",
		jwt.AudienceKey:   []string{"qs-api"},
		"user_id":         "1",
		"tenant_id":       "fangcun",
	})

	result, err := strategy.Verify(context.Background(), token, nil)
	require.NoError(t, err)
	require.True(t, result.Valid)
}

func TestLocalVerifyStrategyEmptyAlgorithmsDefaultsToRS256(t *testing.T) {
	privateKey, manager := newRS256Fixture(t)
	strategy := NewLocalVerifyStrategy(manager, WithLocalConfig(&config.TokenVerifyConfig{}))
	token := signRS256Token(t, privateKey, map[string]interface{}{
		jwt.SubjectKey:    "user:1",
		jwt.ExpirationKey: time.Now().Add(time.Minute),
	})

	result, err := strategy.Verify(context.Background(), token, nil)
	require.NoError(t, err)
	require.True(t, result.Valid)
}

func TestLocalVerifyStrategyRejectsAlgorithmOutsideAllowlist(t *testing.T) {
	privateKey, manager := newRS256Fixture(t)
	strategy := NewLocalVerifyStrategy(manager, WithLocalConfig(&config.TokenVerifyConfig{
		Algorithms: []string{"ES256"},
	}))
	token := signRS256Token(t, privateKey, map[string]interface{}{
		jwt.SubjectKey:    "user:1",
		jwt.ExpirationKey: time.Now().Add(time.Minute),
	})

	result, err := strategy.Verify(context.Background(), token, nil)
	require.Error(t, err)
	require.ErrorIs(t, err, iamerrors.ErrTokenInvalid)
	require.Nil(t, result)
}

func TestNewTokenVerifierRejectsNonRS256Configuration(t *testing.T) {
	_, manager := newRS256Fixture(t)

	result, err := NewTokenVerifier(&config.TokenVerifyConfig{
		AllowedIssuer:   "https://iam.fangcunmount.cn",
		AllowedAudience: []string{"qs-api"},
		Algorithms:      []string{"RS384"},
	}, manager, nil)
	require.Error(t, err)
	require.Nil(t, result)
}

func TestNewTokenVerifierRejectsMissingIssuerOrAudience(t *testing.T) {
	_, manager := newRS256Fixture(t)

	_, err := NewTokenVerifier(nil, manager, nil)
	require.Error(t, err)

	_, err = NewTokenVerifier(&config.TokenVerifyConfig{AllowedAudience: []string{"qs-api"}}, manager, nil)
	require.Error(t, err)

	_, err = NewTokenVerifier(&config.TokenVerifyConfig{AllowedIssuer: "https://iam.fangcunmount.cn"}, manager, nil)
	require.Error(t, err)
}

func TestNewTokenVerifierAcceptsNilAndEmptyAlgorithmConfiguration(t *testing.T) {
	_, manager := newRS256Fixture(t)

	for _, cfg := range []*config.TokenVerifyConfig{
		{AllowedIssuer: "https://iam.fangcunmount.cn", AllowedAudience: []string{"qs-api"}, Algorithms: nil},
		{AllowedIssuer: "https://iam.fangcunmount.cn", AllowedAudience: []string{"qs-api"}, Algorithms: []string{}},
	} {
		result, err := NewTokenVerifier(cfg, manager, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
	}
}

func TestTokenVerifierDoesNotFallbackForExpiredToken(t *testing.T) {
	privateKey, manager := newRS256Fixture(t)
	remote := &verifyTokenClientStub{verifyResp: validRemoteVerifyResponse()}
	verifier, err := NewTokenVerifier(&config.TokenVerifyConfig{
		AllowedIssuer:   "https://iam.fangcunmount.cn",
		AllowedAudience: []string{"qs-api"},
		Algorithms:      []string{"RS256"},
	}, manager, remote)
	require.NoError(t, err)
	token := signRS256Token(t, privateKey, map[string]interface{}{
		jwt.SubjectKey:    "user:1",
		jwt.IssuerKey:     "https://iam.fangcunmount.cn",
		jwt.AudienceKey:   []string{"qs-api"},
		jwt.ExpirationKey: time.Now().Add(-time.Minute),
	})

	result, err := verifier.Verify(context.Background(), token, nil)
	require.ErrorIs(t, err, iamerrors.ErrTokenExpired)
	require.Nil(t, result)
	require.Zero(t, remote.callCount)
}

func TestFallbackVerifyStrategyDoesNotFallbackForInvalidToken(t *testing.T) {
	primary := &verifyStrategyStub{name: "local", err: iamerrors.ErrTokenInvalid}
	fallback := &verifyStrategyStub{
		name: "remote",
		result: &VerifyResult{
			Valid:  true,
			Claims: &TokenClaims{Subject: "remote-subject"},
		},
	}
	strategy := NewFallbackVerifyStrategy(primary, fallback)

	result, err := strategy.Verify(context.Background(), "invalid-token", nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, iamerrors.ErrTokenInvalid))
	require.Nil(t, result)
	require.Equal(t, 1, primary.callCount)
	require.Zero(t, fallback.callCount)
}

func TestFallbackVerifyStrategyFallsBackForJWKSInfrastructureError(t *testing.T) {
	primary := &verifyStrategyStub{
		name: "local",
		err:  allowRemoteFallback(errors.New("jwks unavailable")),
	}
	fallback := &verifyStrategyStub{
		name: "remote",
		result: &VerifyResult{
			Valid:  true,
			Claims: &TokenClaims{Subject: "remote-subject"},
		},
	}
	strategy := NewFallbackVerifyStrategy(primary, fallback)

	result, err := strategy.Verify(context.Background(), "jwt-token", nil)
	require.NoError(t, err)
	require.Equal(t, "remote-subject", result.Claims.Subject)
	require.Equal(t, 1, primary.callCount)
	require.Equal(t, 1, fallback.callCount)
}

func TestRemoteVerifyStrategyEnforcesRequiredClaims(t *testing.T) {
	privateKey, _ := newRS256Fixture(t)
	token := signRS256Token(t, privateKey, map[string]interface{}{
		jwt.SubjectKey:    "user:1",
		jwt.ExpirationKey: time.Now().Add(time.Minute),
		"user_id":         "1",
	})
	stub := &verifyTokenClientStub{verifyResp: validRemoteVerifyResponse()}
	strategy := NewRemoteVerifyStrategy(stub, &config.TokenVerifyConfig{
		RequiredClaims: []string{"sub", "exp", "user_id", "tenant_id"},
		Algorithms:     []string{"RS256"},
	})

	result, err := strategy.Verify(context.Background(), token, nil)
	require.Error(t, err)
	require.ErrorIs(t, err, iamerrors.ErrTokenInvalid)
	require.Nil(t, result)
	require.Zero(t, stub.callCount)
}

func TestRemoteVerifyStrategyEnforcesAlgorithmAllowlist(t *testing.T) {
	privateKey, _ := newRS256Fixture(t)
	token := signRS256Token(t, privateKey, map[string]interface{}{
		jwt.SubjectKey:    "user:1",
		jwt.ExpirationKey: time.Now().Add(time.Minute),
	})
	stub := &verifyTokenClientStub{verifyResp: validRemoteVerifyResponse()}
	strategy := NewRemoteVerifyStrategy(stub, &config.TokenVerifyConfig{
		Algorithms: []string{"ES256"},
	})

	result, err := strategy.Verify(context.Background(), token, nil)
	require.Error(t, err)
	require.ErrorIs(t, err, iamerrors.ErrTokenInvalid)
	require.Nil(t, result)
	require.Zero(t, stub.callCount)
}

func validRemoteVerifyResponse() *authnv2.VerifyTokenResponse {
	return &authnv2.VerifyTokenResponse{
		Valid: true,
		Claims: &authnv2.TokenClaims{
			Subject:   "user:1",
			UserId:    "1",
			TenantId:  "fangcun",
			ExpiresAt: timestamppb.New(time.Now().Add(time.Minute)),
		},
	}
}
