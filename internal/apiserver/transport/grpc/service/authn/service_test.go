package authn

import (
	"context"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authnv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authn/v2"
	linkingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/linking"
	sessionApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/session"
	signupApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signup"
	tokenApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
)

type tokenServiceStub struct {
	issueReq   tokenApp.IssueServiceTokenRequest
	issueRes   *tokenApp.TokenIssueResult
	issueErr   error
	verifyReq  tokenApp.VerifyTokenRequest
	verifyErr  error
	refreshErr error
	revokeErr  error
}

type loginServiceStub struct {
	req        sessionApp.LoginRequest
	res        *sessionApp.LoginResult
	err        error
	refreshErr error
}

func (s *loginServiceStub) Login(ctx context.Context, req sessionApp.LoginRequest) (*sessionApp.LoginResult, error) {
	s.req = req
	return s.res, s.err
}

func (s *loginServiceStub) RenewSession(context.Context, string) (*sessionApp.RenewResult, error) {
	if s.refreshErr != nil {
		return nil, s.refreshErr
	}
	if s.res != nil {
		return &sessionApp.RenewResult{TokenPair: s.res.TokenPair}, nil
	}
	return &sessionApp.RenewResult{}, nil
}

func (s *loginServiceStub) Logout(ctx context.Context, req sessionApp.LogoutRequest) error {
	return nil
}

type signupServiceStub struct{}

func (s signupServiceStub) SignUp(context.Context, signupApp.SignupRequest) (*signupApp.SignupResult, error) {
	return &signupApp.SignupResult{}, nil
}

type challengeServiceStub struct{}

func (s challengeServiceStub) SendLoginPhoneOTP(context.Context, string) error {
	return nil
}

func (s challengeServiceStub) SendPhoneLinkOTP(context.Context, string) error {
	return nil
}

type linkerStub struct{}

func (s linkerStub) List(context.Context, meta.ID) ([]linkingApp.LoginIdentityView, error) {
	return nil, nil
}
func (s linkerStub) Link(context.Context, linkingApp.LinkRequest) (*linkingApp.LinkResult, error) {
	return &linkingApp.LinkResult{}, nil
}
func (s linkerStub) Unlink(context.Context, linkingApp.UnlinkCommand) error {
	return nil
}

func (s *tokenServiceStub) IssueToken(ctx context.Context, principal *authentication.Principal) (*tokenApp.TokenPair, error) {
	return nil, nil
}

func (s *tokenServiceStub) IssueServiceToken(ctx context.Context, req tokenApp.IssueServiceTokenRequest) (*tokenApp.TokenIssueResult, error) {
	s.issueReq = req
	return s.issueRes, s.issueErr
}

func (s *tokenServiceStub) RefreshToken(ctx context.Context, refreshToken string) (*tokenApp.TokenRefreshResult, error) {
	return nil, s.refreshErr
}

func (s *tokenServiceStub) RevokeAccessToken(ctx context.Context, accessToken string) error {
	return s.revokeErr
}

func (s *tokenServiceStub) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	return nil
}

func (s *tokenServiceStub) VerifyToken(ctx context.Context, req tokenApp.VerifyTokenRequest) (*tokenApp.TokenVerifyResult, error) {
	s.verifyReq = req
	if s.verifyErr != nil {
		return nil, s.verifyErr
	}
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
			"fangcun",
			"iam",
			[]string{"test"},
			map[string]string{"scope": "internal", "level": "2"},
			[]string{"pwd"},
			time.Now(),
			time.Now().Add(time.Minute),
		),
	}, nil
}

func TestAuthNGRPCRuntimeRegistersProductionServices(t *testing.T) {
	server := grpc.NewServer()
	NewService(
		&loginServiceStub{},
		&tokenServiceStub{},
		signupServiceStub{},
		challengeServiceStub{},
		challengeServiceStub{},
		linkerStub{},
		nil,
	).Register(server)

	info := server.GetServiceInfo()
	require.Contains(t, info, "iam.authn.v2.AuthService")
	require.Contains(t, info, "iam.authn.v2.AuthSignupService")
	require.Contains(t, info, "iam.authn.v2.AuthChallengeService")
	require.Contains(t, info, "iam.authn.v2.LoginIdentityService")
	require.NotContains(t, info, "iam.authn.v2.AccountOnboardingService")
}

func TestAuthServiceServerLoginUsesExplicitV2Contract(t *testing.T) {
	access := tokenApp.NewAccessToken("access-id", "access-token", "session-id", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(7), time.Hour)
	refresh := tokenApp.NewRefreshToken("refresh-id", "refresh-token", "session-id", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(7), []string{"pwd"}, nil, 24*time.Hour)
	stub := &loginServiceStub{
		res: &sessionApp.LoginResult{
			TokenPair:       tokenApp.NewTokenPair(access, refresh),
			UserID:          meta.FromUint64(1),
			LoginIdentityID: meta.FromUint64(2),
			TenantID:        meta.FromUint64(7),
		},
	}
	srv := &authServiceServer{sessionSvc: stub}
	payload, err := structpb.NewStruct(map[string]any{
		"username":  "alice",
		"password":  "secret",
		"tenant_id": 7,
	})
	require.NoError(t, err)

	resp, err := srv.Login(context.Background(), &authnv2.LoginRequest{
		AuthMethod:    "password",
		MethodPayload: payload,
	})

	require.NoError(t, err)
	require.NotNil(t, resp.GetTokenPair())
	require.Equal(t, "access-token", resp.GetTokenPair().GetAccessToken())
	require.Equal(t, "refresh-token", resp.GetTokenPair().GetRefreshToken())
	require.Equal(t, sessionApp.AuthMethodPassword, stub.req.AuthMethod)
	require.Equal(t, meta.FromUint64(7), stub.req.TenantID)
	loginPayload, ok := stub.req.Payload.(sessionApp.PasswordPayload)
	require.True(t, ok)
	require.Equal(t, "alice", loginPayload.Username)
	require.Equal(t, "secret", loginPayload.Password)
}

func TestAuthServiceServerLoginRejectsNonPublicMethod(t *testing.T) {
	srv := &authServiceServer{sessionSvc: &loginServiceStub{}}
	payload, err := structpb.NewStruct(map[string]any{"token": "jwt"})
	require.NoError(t, err)

	_, err = srv.Login(context.Background(), &authnv2.LoginRequest{
		AuthMethod:    "jwt_token",
		MethodPayload: payload,
	})

	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
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

	resp, err := srv.IssueServiceToken(context.Background(), &authnv2.IssueServiceTokenRequest{
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

	_, err := srv.IssueServiceToken(context.Background(), &authnv2.IssueServiceTokenRequest{})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestAuthServiceServerVerifyTokenPassesExpectationGuards(t *testing.T) {
	stub := &tokenServiceStub{}
	srv := &authServiceServer{tokenSvc: stub}

	_, err := srv.VerifyToken(context.Background(), &authnv2.VerifyTokenRequest{
		AccessToken:      "jwt-token",
		ExpectedIssuer:   "https://iam.fangcunmount.cn",
		ExpectedAudience: []string{"qs-api"},
	})
	require.NoError(t, err)
	require.Equal(t, "jwt-token", stub.verifyReq.AccessToken)
	require.Equal(t, "https://iam.fangcunmount.cn", stub.verifyReq.ExpectedIssuer)
	require.Equal(t, []string{"qs-api"}, stub.verifyReq.ExpectedAudience)
}

func TestAuthServiceServerTokenLifecycleErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		call       func(*authServiceServer) error
		sessionSvc sessionApp.ApplicationService
		tokenSvc   *tokenServiceStub
		want       codes.Code
	}{
		{
			name: "verify token app unauthenticated",
			call: func(s *authServiceServer) error {
				_, err := s.VerifyToken(context.Background(), &authnv2.VerifyTokenRequest{AccessToken: "access-token"})
				return err
			},
			tokenSvc: &tokenServiceStub{verifyErr: perrors.WithCode(code.ErrTokenInvalid, "invalid access")},
			want:     codes.Unauthenticated,
		},
		{
			name: "refresh token app unauthenticated",
			call: func(s *authServiceServer) error {
				_, err := s.RefreshToken(context.Background(), &authnv2.RefreshTokenRequest{RefreshToken: "refresh-token"})
				return err
			},
			sessionSvc: &loginServiceStub{refreshErr: perrors.WithCode(code.ErrTokenInvalid, "invalid refresh")},
			want:       codes.Unauthenticated,
		},
		{
			name: "revoke token app unauthenticated",
			call: func(s *authServiceServer) error {
				_, err := s.RevokeToken(context.Background(), &authnv2.RevokeTokenRequest{AccessToken: "access-token"})
				return err
			},
			tokenSvc: &tokenServiceStub{revokeErr: perrors.WithCode(code.ErrTokenInvalid, "invalid access")},
			want:     codes.Unauthenticated,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			srv := &authServiceServer{sessionSvc: tc.sessionSvc, tokenSvc: tc.tokenSvc}

			err := tc.call(srv)

			require.Error(t, err)
			require.Equal(t, tc.want, status.Code(err))
		})
	}
}
