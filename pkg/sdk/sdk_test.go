package sdk

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	authnv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/authn/v2"
	authclient "github.com/FangcunMount/iam/v3/pkg/sdk/auth/client"
	authjwks "github.com/FangcunMount/iam/v3/pkg/sdk/auth/jwks"
	authserviceauth "github.com/FangcunMount/iam/v3/pkg/sdk/auth/serviceauth"
	authverifier "github.com/FangcunMount/iam/v3/pkg/sdk/auth/verifier"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type sdkAuthServiceClientStub struct {
	verifyReq  *authnv2.VerifyTokenRequest
	verifyResp *authnv2.VerifyTokenResponse
	verifyErr  error

	issueResp *authnv2.IssueServiceTokenResponse
	issueErr  error
}

func (s *sdkAuthServiceClientStub) VerifyToken(ctx context.Context, in *authnv2.VerifyTokenRequest, _ ...grpc.CallOption) (*authnv2.VerifyTokenResponse, error) {
	s.verifyReq = in
	return s.verifyResp, s.verifyErr
}

func (s *sdkAuthServiceClientStub) Login(context.Context, *authnv2.LoginRequest, ...grpc.CallOption) (*authnv2.LoginResponse, error) {
	return nil, nil
}

func (s *sdkAuthServiceClientStub) RefreshToken(context.Context, *authnv2.RefreshTokenRequest, ...grpc.CallOption) (*authnv2.RefreshTokenResponse, error) {
	return nil, nil
}

func (s *sdkAuthServiceClientStub) RevokeToken(context.Context, *authnv2.RevokeTokenRequest, ...grpc.CallOption) (*authnv2.RevokeTokenResponse, error) {
	return nil, nil
}

func (s *sdkAuthServiceClientStub) RevokeRefreshToken(context.Context, *authnv2.RevokeRefreshTokenRequest, ...grpc.CallOption) (*authnv2.RevokeRefreshTokenResponse, error) {
	return nil, nil
}

func (s *sdkAuthServiceClientStub) IssueServiceToken(context.Context, *authnv2.IssueServiceTokenRequest, ...grpc.CallOption) (*authnv2.IssueServiceTokenResponse, error) {
	return s.issueResp, s.issueErr
}

type sdkJWKSServiceClientStub struct {
	resp *authnv2.GetJWKSResponse
	err  error
}

func (s *sdkJWKSServiceClientStub) GetJWKS(context.Context, *authnv2.GetJWKSRequest, ...grpc.CallOption) (*authnv2.GetJWKSResponse, error) {
	return s.resp, s.err
}

func TestClientAuthUsesTypedAuthClient(t *testing.T) {
	t.Parallel()

	authStub := &sdkAuthServiceClientStub{
		verifyResp: &authnv2.VerifyTokenResponse{
			Valid: true,
			Claims: &authnv2.TokenClaims{
				TokenId:   "jti-1",
				Subject:   "user:1",
				SessionId: "sid-1",
				UserId:    "1",
				Issuer:    "https://iam.example.com",
				Audience:  []string{"qs-api"},
				IssuedAt:  timestamppb.New(time.Now()),
				ExpiresAt: timestamppb.New(time.Now().Add(time.Minute)),
			},
		},
	}

	client := &Client{
		authClient: authclient.NewClient(authStub, &sdkJWKSServiceClientStub{}),
	}

	resp, err := client.Auth().VerifyToken(context.Background(), &authnv2.VerifyTokenRequest{
		AccessToken: "jwt-token",
	})
	require.NoError(t, err)
	require.True(t, resp.GetValid())
	require.NotNil(t, authStub.verifyReq)
}

func TestAuthSubpackagesComposeWithSDKClient(t *testing.T) {
	t.Parallel()

	authStub := &sdkAuthServiceClientStub{
		verifyResp: &authnv2.VerifyTokenResponse{
			Valid: true,
			Claims: &authnv2.TokenClaims{
				TokenId:   "jti-1",
				Subject:   "user:1",
				SessionId: "sid-1",
				UserId:    "1",
				Issuer:    "https://iam.example.com",
				Audience:  []string{"qs-api"},
				IssuedAt:  timestamppb.New(time.Now()),
				ExpiresAt: timestamppb.New(time.Now().Add(time.Minute)),
			},
		},
		issueResp: &authnv2.IssueServiceTokenResponse{
			TokenPair: &authnv2.TokenPair{
				AccessToken: "svc-token",
				ExpiresIn:   durationpb.New(time.Minute),
			},
		},
	}

	client := &Client{
		authClient: authclient.NewClient(authStub, &sdkJWKSServiceClientStub{
			resp: &authnv2.GetJWKSResponse{Jwks: []byte(`{"keys":[]}`)},
		}),
	}

	jwksManager, err := authjwks.NewJWKSManager(&JWKSConfig{
		URL:            "https://iam.example.com/.well-known/jwks.json",
		RequestTimeout: time.Second,
	}, authjwks.WithAuthClient(client.Auth()), authjwks.WithSeedData([]byte(`{"keys":[]}`)))
	require.NoError(t, err)
	defer jwksManager.Stop()

	verifier, err := authverifier.NewTokenVerifier(&TokenVerifyConfig{
		AllowedIssuer:   "https://iam.example.com",
		AllowedAudience: []string{"qs-api"},
	}, jwksManager, client.Auth())
	require.NoError(t, err)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwtToken := jwt.New()
	require.NoError(t, jwtToken.Set(jwt.SubjectKey, "user:1"))
	require.NoError(t, jwtToken.Set(jwt.IssuerKey, "https://iam.example.com"))
	require.NoError(t, jwtToken.Set(jwt.AudienceKey, []string{"qs-api"}))
	require.NoError(t, jwtToken.Set(jwt.ExpirationKey, time.Now().Add(time.Minute)))
	signedToken, err := jwt.Sign(jwtToken, jwt.WithKey(jwa.RS256, privateKey))
	require.NoError(t, err)

	result, err := verifier.Verify(context.Background(), string(signedToken), nil)
	require.NoError(t, err)
	require.Equal(t, "sid-1", result.Claims.SessionID)

	helper, err := authserviceauth.NewServiceAuthHelper(&ServiceAuthConfig{
		ServiceID:      "qs-service",
		TargetAudience: []string{"iam-service"},
		TokenTTL:       time.Minute,
		RefreshBefore:  5 * time.Second,
	}, client.Auth())
	require.NoError(t, err)
	defer helper.Stop()

	token, err := helper.GetToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, "svc-token", token)
}
