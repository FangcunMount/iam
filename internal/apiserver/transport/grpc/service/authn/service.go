package authn

import (
	authnv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authn/v2"
	challengeApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/challenge"
	jwksApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/jwks"
	linkingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/linking"
	loginApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login"
	signupApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signup"
	tokenApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"google.golang.org/grpc"
)

// Service 聚合 authn 模块的 gRPC 服务
type Service struct {
	auth          authServiceServer
	signup        authSignupServiceServer
	challenge     authChallengeServiceServer
	loginIdentity loginIdentityServiceServer
	jwks          jwksServiceServer
}

// NewService 创建 authn gRPC 服务
func NewService(
	loginSvc loginApp.LoginApplicationService,
	tokenSvc tokenApp.TokenApplicationService,
	signupSvc signupApp.SignupService,
	challengeSvc challengeApp.Service,
	linkingSvc linkingApp.Service,
	keyPublish *jwksApp.KeyPublishAppService,
) *Service {
	return &Service{
		auth: authServiceServer{
			loginSvc: loginSvc,
			tokenSvc: tokenSvc,
		},
		signup: authSignupServiceServer{
			signupService: signupSvc,
		},
		challenge: authChallengeServiceServer{
			challenge: challengeSvc,
		},
		loginIdentity: loginIdentityServiceServer{
			linking: linkingSvc,
		},
		jwks: jwksServiceServer{
			keyPublish: keyPublish,
		},
	}
}

// Register 注册 gRPC 服务
func (s *Service) Register(server *grpc.Server) {
	if s == nil || server == nil {
		return
	}
	if s.auth.loginSvc != nil || s.auth.tokenSvc != nil {
		authnv2.RegisterAuthServiceServer(server, &s.auth)
	}
	if s.signup.signupService != nil {
		authnv2.RegisterAuthSignupServiceServer(server, &s.signup)
	}
	if s.challenge.challenge != nil {
		authnv2.RegisterAuthChallengeServiceServer(server, &s.challenge)
	}
	if s.loginIdentity.linking != nil {
		authnv2.RegisterLoginIdentityServiceServer(server, &s.loginIdentity)
	}
	if s.jwks.keyPublish != nil {
		authnv2.RegisterJWKSServiceServer(server, &s.jwks)
	}
}

type authServiceServer struct {
	authnv2.UnimplementedAuthServiceServer
	loginSvc loginApp.LoginApplicationService
	tokenSvc tokenApp.TokenApplicationService
}

type jwksServiceServer struct {
	authnv2.UnimplementedJWKSServiceServer
	keyPublish *jwksApp.KeyPublishAppService
}

type authSignupServiceServer struct {
	authnv2.UnimplementedAuthSignupServiceServer
	signupService signupApp.SignupService
}

type authChallengeServiceServer struct {
	authnv2.UnimplementedAuthChallengeServiceServer
	challenge challengeApp.Service
}

type loginIdentityServiceServer struct {
	authnv2.UnimplementedLoginIdentityServiceServer
	linking linkingApp.Service
}
