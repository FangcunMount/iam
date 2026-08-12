package authn

import (
	authnv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/authn/v2"
	challengeApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/challenge"
	jwksApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/jwks"
	linkingApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/linking"
	sessionApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/session"
	signupApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signup"
	tokenApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/token"
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
	sessionSvc sessionApp.ApplicationService,
	tokenSvc tokenApp.TokenApplicationService,
	signupSvc signupApp.SignupService,
	loginPhoneOTPSender challengeApp.LoginPhoneOTPSender,
	phoneLinkOTPSender challengeApp.PhoneLinkOTPSender,
	linkingSvc linkingApp.Linker,
	keyPublish *jwksApp.KeyPublishAppService,
) *Service {
	return &Service{
		auth: authServiceServer{
			sessionSvc: sessionSvc,
			tokenSvc:   tokenSvc,
		},
		signup: authSignupServiceServer{
			signupService: signupSvc,
		},
		challenge: authChallengeServiceServer{
			loginPhoneOTPSender: loginPhoneOTPSender,
		},
		loginIdentity: loginIdentityServiceServer{
			linking:            linkingSvc,
			phoneLinkOTPSender: phoneLinkOTPSender,
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
	if s.auth.sessionSvc != nil || s.auth.tokenSvc != nil {
		authnv2.RegisterAuthServiceServer(server, &s.auth)
	}
	if s.signup.signupService != nil {
		authnv2.RegisterAuthSignupServiceServer(server, &s.signup)
	}
	if s.challenge.loginPhoneOTPSender != nil {
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
	sessionSvc sessionApp.ApplicationService
	tokenSvc   tokenApp.TokenApplicationService
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
	loginPhoneOTPSender challengeApp.LoginPhoneOTPSender
}

type loginIdentityServiceServer struct {
	authnv2.UnimplementedLoginIdentityServiceServer
	linking            linkingApp.Linker
	phoneLinkOTPSender challengeApp.PhoneLinkOTPSender
}
