package verifier

import (
	"context"
	"testing"
	"time"

	authnv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authn/v2"
	"github.com/FangcunMount/iam/v2/pkg/sdk/config"
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
}

func (s *verifyTokenClientStub) VerifyToken(_ context.Context, in *authnv2.VerifyTokenRequest) (*authnv2.VerifyTokenResponse, error) {
	s.verifyReq = in
	return s.verifyResp, s.verifyErr
}

func TestRemoteVerifyStrategyPassesConfiguredIssuerAndAudience(t *testing.T) {
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

	_, err := strategy.Verify(context.Background(), "jwt-token", nil)
	require.NoError(t, err)
	require.NotNil(t, stub.verifyReq)
	require.Equal(t, "https://iam.fangcunmount.cn", stub.verifyReq.ExpectedIssuer)
	require.Equal(t, []string{"qs-api"}, stub.verifyReq.ExpectedAudience)
}

func TestRemoteVerifyStrategyOptionsOverrideConfig(t *testing.T) {
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

	_, err := strategy.Verify(context.Background(), "jwt-token", &VerifyOptions{
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

func TestRemoteVerifyStrategyReturnsSessionID(t *testing.T) {
	stub := &verifyTokenClientStub{
		verifyResp: &authnv2.VerifyTokenResponse{
			Valid: true,
			Claims: &authnv2.TokenClaims{
				TokenId:         "jti-remote",
				Subject:         "user:1",
				SessionId:       "sid-remote",
				UserId:          "1",
				LoginIdentityId: "2",
				TenantId:        "3",
				Issuer:          "https://iam.fangcunmount.cn",
				Audience:        []string{"qs-api"},
				Amr:             []string{"pwd", "otp"},
				IssuedAt:        timestamppb.New(time.Now()),
				ExpiresAt:       timestamppb.New(time.Now().Add(time.Minute)),
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

	result, err := strategy.Verify(context.Background(), "jwt-token", nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Claims)
	require.Equal(t, "jti-remote", result.Claims.TokenID)
	require.Equal(t, "sid-remote", result.Claims.SessionID)
	require.Equal(t, []string{"pwd", "otp"}, result.Claims.AMR)
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
	require.NoError(t, token.Set("tenant_id", "3"))

	claims := extractClaims(token)
	require.NotNil(t, claims)
	require.Equal(t, "jti-local", claims.TokenID)
	require.Equal(t, "sid-local", claims.SessionID)
	require.Equal(t, "1", claims.UserID)
	require.Equal(t, "2", claims.LoginIdentityID)
	require.Equal(t, "3", claims.TenantID)
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
