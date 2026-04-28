package sdk

import (
	"context"
	"testing"
	"time"

	authnv1 "github.com/FangcunMount/iam/api/grpc/iam/authn/v1"
	authclient "github.com/FangcunMount/iam/pkg/sdk/auth/client"
	authjwks "github.com/FangcunMount/iam/pkg/sdk/auth/jwks"
	authserviceauth "github.com/FangcunMount/iam/pkg/sdk/auth/serviceauth"
	authverifier "github.com/FangcunMount/iam/pkg/sdk/auth/verifier"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type sdkAuthServiceClientStub struct {
	verifyReq  *authnv1.VerifyTokenRequest
	verifyResp *authnv1.VerifyTokenResponse
	verifyErr  error

	issueResp *authnv1.IssueServiceTokenResponse
	issueErr  error
}

func (s *sdkAuthServiceClientStub) VerifyToken(ctx context.Context, in *authnv1.VerifyTokenRequest, _ ...grpc.CallOption) (*authnv1.VerifyTokenResponse, error) {
	s.verifyReq = in
	return s.verifyResp, s.verifyErr
}

func (s *sdkAuthServiceClientStub) RegisterOperationAccount(context.Context, *authnv1.RegisterOperationAccountRequest, ...grpc.CallOption) (*authnv1.RegisterOperationAccountResponse, error) {
	return nil, nil
}

func (s *sdkAuthServiceClientStub) RefreshToken(context.Context, *authnv1.RefreshTokenRequest, ...grpc.CallOption) (*authnv1.RefreshTokenResponse, error) {
	return nil, nil
}

func (s *sdkAuthServiceClientStub) RevokeToken(context.Context, *authnv1.RevokeTokenRequest, ...grpc.CallOption) (*authnv1.RevokeTokenResponse, error) {
	return nil, nil
}

func (s *sdkAuthServiceClientStub) RevokeRefreshToken(context.Context, *authnv1.RevokeRefreshTokenRequest, ...grpc.CallOption) (*authnv1.RevokeRefreshTokenResponse, error) {
	return nil, nil
}

func (s *sdkAuthServiceClientStub) IssueServiceToken(context.Context, *authnv1.IssueServiceTokenRequest, ...grpc.CallOption) (*authnv1.IssueServiceTokenResponse, error) {
	return s.issueResp, s.issueErr
}

type sdkJWKSServiceClientStub struct {
	resp *authnv1.GetJWKSResponse
	err  error
}

func (s *sdkJWKSServiceClientStub) GetJWKS(context.Context, *authnv1.GetJWKSRequest, ...grpc.CallOption) (*authnv1.GetJWKSResponse, error) {
	return s.resp, s.err
}

func TestClientAuthUsesTypedAuthClient(t *testing.T) {
	t.Parallel()

	authStub := &sdkAuthServiceClientStub{
		verifyResp: &authnv1.VerifyTokenResponse{
			Valid: true,
			Claims: &authnv1.TokenClaims{
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

	resp, err := client.Auth().VerifyToken(context.Background(), &authnv1.VerifyTokenRequest{
		AccessToken: "jwt-token",
	})
	require.NoError(t, err)
	require.True(t, resp.GetValid())
	require.NotNil(t, authStub.verifyReq)
}

func TestAuthSubpackagesComposeWithSDKClient(t *testing.T) {
	t.Parallel()

	authStub := &sdkAuthServiceClientStub{
		verifyResp: &authnv1.VerifyTokenResponse{
			Valid: true,
			Claims: &authnv1.TokenClaims{
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
		issueResp: &authnv1.IssueServiceTokenResponse{
			TokenPair: &authnv1.TokenPair{
				AccessToken: "svc-token",
				ExpiresIn:   durationpb.New(time.Minute),
			},
		},
	}

	client := &Client{
		authClient: authclient.NewClient(authStub, &sdkJWKSServiceClientStub{
			resp: &authnv1.GetJWKSResponse{Jwks: []byte(`{"keys":[]}`)},
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

	result, err := verifier.Verify(context.Background(), "jwt-token", nil)
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
