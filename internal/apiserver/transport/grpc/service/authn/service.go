package authn

import (
	authnv3 "github.com/FangcunMount/iam/v5/api/grpc/iam/authn/v3"
	challengeApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authn/challenge"
	jwksApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authn/jwks"
	linkingApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authn/linking"
	notificationrecipient "github.com/FangcunMount/iam/v5/internal/apiserver/application/authn/notificationrecipient"
	sessionApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authn/session"
	signupApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authn/signup"
	tokenApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authn/token"
	"google.golang.org/grpc"
)

// Service 聚合 authn 模块的 gRPC 服务
type Service struct {
	auth                  authServiceServer
	signup                authSignupServiceServer
	challenge             authChallengeServiceServer
	loginIdentity         loginIdentityServiceServer
	notificationRecipient notificationRecipientServiceServer
	jwks                  jwksServiceServer
}

// NewService 创建 authn gRPC 服务
func NewService(
	sessionSvc sessionApp.ApplicationService,
	tokens tokenApp.Capabilities,
	signupSvc signupApp.SignupService,
	loginPhoneOTPSender challengeApp.LoginPhoneOTPSender,
	phoneLinkOTPSender challengeApp.PhoneLinkOTPSender,
	linkingSvc linkingApp.Linker,
	keyPublish *jwksApp.KeyPublishAppService,
	recipientQuery *notificationrecipient.Query,
	notificationRecipientAppIDs []string,
) *Service {
	return &Service{
		auth: authServiceServer{
			sessionSvc:    sessionSvc,
			tokenVerifier: tokens.Verifier,
			tokenRevoker:  tokens.Revoker,
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
		notificationRecipient: notificationRecipientServiceServer{
			recipientQuery: recipientQuery,
			allowedAppIDs:  newNotificationAppIDSet(notificationRecipientAppIDs),
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
	if s.auth.sessionSvc != nil || s.auth.tokenVerifier != nil || s.auth.tokenRevoker != nil {
		authnv3.RegisterAuthServiceServer(server, &s.auth)
	}
	if s.signup.signupService != nil {
		authnv3.RegisterAuthSignupServiceServer(server, &s.signup)
	}
	if s.challenge.loginPhoneOTPSender != nil {
		authnv3.RegisterAuthChallengeServiceServer(server, &s.challenge)
	}
	if s.loginIdentity.linking != nil {
		authnv3.RegisterLoginIdentityServiceServer(server, &s.loginIdentity)
	}
	if s.notificationRecipient.recipientQuery != nil {
		authnv3.RegisterNotificationRecipientServiceServer(server, &s.notificationRecipient)
	}
	if s.jwks.keyPublish != nil {
		authnv3.RegisterJWKSServiceServer(server, &s.jwks)
	}
}

type authServiceServer struct {
	authnv3.UnimplementedAuthServiceServer
	sessionSvc    sessionApp.ApplicationService
	tokenVerifier tokenApp.Verifier
	tokenRevoker  tokenApp.Revoker
}

type jwksServiceServer struct {
	authnv3.UnimplementedJWKSServiceServer
	keyPublish *jwksApp.KeyPublishAppService
}

type authSignupServiceServer struct {
	authnv3.UnimplementedAuthSignupServiceServer
	signupService signupApp.SignupService
}

type authChallengeServiceServer struct {
	authnv3.UnimplementedAuthChallengeServiceServer
	loginPhoneOTPSender challengeApp.LoginPhoneOTPSender
}

type loginIdentityServiceServer struct {
	authnv3.UnimplementedLoginIdentityServiceServer
	linking            linkingApp.Linker
	phoneLinkOTPSender challengeApp.PhoneLinkOTPSender
}
