package authn

import (
	"context"
	"testing"
	"time"

	authnv1 "github.com/FangcunMount/iam/api/grpc/iam/authn/v1"
	onboardingApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/onboarding"
	tokenApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	accountDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/account"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
)

type tokenServiceStub struct {
	issueReq  tokenApp.IssueServiceTokenRequest
	issueRes  *tokenApp.TokenIssueResult
	issueErr  error
	verifyReq tokenApp.VerifyTokenRequest
}

type accountOnboarderStub struct {
	req onboardingApp.OnboardingRequest
	res *onboardingApp.OnboardingResult
	err error
}

func (s *tokenServiceStub) IssueServiceToken(ctx context.Context, req tokenApp.IssueServiceTokenRequest) (*tokenApp.TokenIssueResult, error) {
	s.issueReq = req
	return s.issueRes, s.issueErr
}

func (s *tokenServiceStub) RefreshToken(ctx context.Context, refreshToken string) (*tokenApp.TokenRefreshResult, error) {
	return nil, nil
}

func (s *tokenServiceStub) RevokeAccessToken(ctx context.Context, accessToken string) error {
	return nil
}

func (s *tokenServiceStub) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	return nil
}

func (s *tokenServiceStub) VerifyToken(ctx context.Context, req tokenApp.VerifyTokenRequest) (*tokenApp.TokenVerifyResult, error) {
	s.verifyReq = req
	return &tokenApp.TokenVerifyResult{
		Valid: true,
		Claims: tokenApp.NewTokenClaims(
			tokenApp.TokenTypeAccess,
			"tid",
			"user:1",
			"sid-1",
			meta.FromUint64(1),
			meta.FromUint64(2),
			meta.FromUint64(3),
			"iam",
			[]string{"test"},
			map[string]string{"scope": "internal", "level": "2"},
			[]string{"pwd"},
			time.Now(),
			time.Now().Add(time.Minute),
		),
	}, nil
}

func (s *accountOnboarderStub) Onboard(ctx context.Context, req onboardingApp.OnboardingRequest) (*onboardingApp.OnboardingResult, error) {
	s.req = req
	return s.res, s.err
}

func TestAuthServiceServerIssueServiceToken(t *testing.T) {
	serviceToken := tokenApp.NewServiceToken("sid", "jwt-service-token", "service:qs-server", []string{"iam-service"}, map[string]string{"scope": "internal"}, time.Hour)
	stub := &tokenServiceStub{
		issueRes: &tokenApp.TokenIssueResult{
			TokenPair: tokenApp.NewTokenPair(serviceToken, nil),
		},
	}
	srv := &authServiceServer{tokenSvc: stub}

	attrs, err := structpb.NewStruct(map[string]any{"scope": "internal", "level": 2})
	require.NoError(t, err)

	resp, err := srv.IssueServiceToken(context.Background(), &authnv1.IssueServiceTokenRequest{
		Subject:    "service:qs-server",
		Audience:   []string{"iam-service"},
		Ttl:        durationpb.New(time.Hour),
		Attributes: attrs,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.TokenPair)
	require.Equal(t, "jwt-service-token", resp.TokenPair.AccessToken)
	require.Equal(t, "Bearer", resp.TokenPair.TokenType)
	require.Equal(t, "service:qs-server", stub.issueReq.Subject)
	require.Equal(t, []string{"iam-service"}, stub.issueReq.Audience)
	require.Equal(t, time.Hour, stub.issueReq.TTL)
	require.Equal(t, "internal", stub.issueReq.Attributes["scope"])
	require.Equal(t, "2", stub.issueReq.Attributes["level"])
}

func TestAuthServiceServerIssueServiceTokenValidation(t *testing.T) {
	srv := &authServiceServer{tokenSvc: &tokenServiceStub{}}

	_, err := srv.IssueServiceToken(context.Background(), &authnv1.IssueServiceTokenRequest{})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestAuthServiceServerVerifyTokenPassesExpectationGuards(t *testing.T) {
	stub := &tokenServiceStub{}
	srv := &authServiceServer{tokenSvc: stub}

	_, err := srv.VerifyToken(context.Background(), &authnv1.VerifyTokenRequest{
		AccessToken:      "jwt-token",
		ExpectedIssuer:   "https://iam.fangcunmount.cn",
		ExpectedAudience: []string{"qs-api"},
	})
	require.NoError(t, err)
	require.Equal(t, "jwt-token", stub.verifyReq.AccessToken)
	require.Equal(t, "https://iam.fangcunmount.cn", stub.verifyReq.ExpectedIssuer)
	require.Equal(t, []string{"qs-api"}, stub.verifyReq.ExpectedAudience)
}

func TestAccountOnboardingServiceServerCreateOperationAccount(t *testing.T) {
	password := "Secret123!"
	stub := &accountOnboarderStub{
		res: &onboardingApp.OnboardingResult{
			UserID:       meta.FromUint64(101),
			AccountID:    meta.FromUint64(202),
			AccountType:  accountDomain.TypeOpera,
			ExternalID:   accountDomain.ExternalID("staff@example.com"),
			CredentialID: meta.FromUint64(303),
			IsNewUser:    true,
			IsNewAccount: true,
		},
	}
	srv := &accountOnboardingServer{accountOnboarder: stub}

	resp, err := srv.CreateOperationAccount(context.Background(), &authnv1.CreateOperationAccountRequest{
		Name:           "张三",
		Phone:          "13800138000",
		Email:          "staff@example.com",
		ScopedTenantId: "1",
		Password:       password,
	})
	require.NoError(t, err)
	require.Equal(t, "101", resp.GetUserId())
	require.Equal(t, "202", resp.GetAccountId())
	require.Equal(t, "303", resp.GetCredentialId())
	require.Equal(t, "staff@example.com", resp.GetExternalId())
	require.Equal(t, accountDomain.TypeOpera, stub.req.AccountType)
	require.Equal(t, onboardingApp.CredTypePassword, stub.req.CredentialType)
	require.NotNil(t, stub.req.Password)
	require.Equal(t, password, *stub.req.Password)
	require.Equal(t, meta.FromUint64(1), stub.req.ScopedTenantID)
}
