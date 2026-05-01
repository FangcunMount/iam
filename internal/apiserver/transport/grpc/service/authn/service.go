package authn

import (
	authnv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authn/v2"
	jwksApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/jwks"
	onboardingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/onboarding"
	tokenApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"google.golang.org/grpc"
)

// Service 聚合 authn 模块的 gRPC 服务
type Service struct {
	auth       authServiceServer
	onboarding accountOnboardingServer
	jwks       jwksServiceServer
}

// NewService 创建 authn gRPC 服务
func NewService(
	tokenSvc tokenApp.TokenApplicationService,
	accountOnboarder onboardingApp.AccountOnboarder,
	keyPublish *jwksApp.KeyPublishAppService,
) *Service {
	return &Service{
		auth: authServiceServer{
			tokenSvc: tokenSvc,
		},
		onboarding: accountOnboardingServer{
			accountOnboarder: accountOnboarder,
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
	if s.auth.tokenSvc != nil {
		authnv2.RegisterAuthServiceServer(server, &s.auth)
	}
	if s.onboarding.accountOnboarder != nil {
		authnv2.RegisterAccountOnboardingServiceServer(server, &s.onboarding)
	}
	if s.jwks.keyPublish != nil {
		authnv2.RegisterJWKSServiceServer(server, &s.jwks)
	}
}

type authServiceServer struct {
	authnv2.UnimplementedAuthServiceServer
	tokenSvc tokenApp.TokenApplicationService
}

type accountOnboardingServer struct {
	authnv2.UnimplementedAccountOnboardingServiceServer
	accountOnboarder onboardingApp.AccountOnboarder
}

type jwksServiceServer struct {
	authnv2.UnimplementedJWKSServiceServer
	keyPublish *jwksApp.KeyPublishAppService
}
