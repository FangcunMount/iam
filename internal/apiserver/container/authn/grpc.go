package authn

import (
	grpctransport "github.com/FangcunMount/iam/v3/internal/apiserver/transport/grpc"
	authngrpc "github.com/FangcunMount/iam/v3/internal/apiserver/transport/grpc/service/authn"
)

// CollectGRPC appends authn gRPC registration when the module is available.
func CollectGRPC(available bool, mod *AuthnModule, registrations *[]grpctransport.Registration) {
	if !available || mod == nil || registrations == nil {
		return
	}
	caps := mod.ApplicationCapabilities()
	service := authngrpc.NewService(
		caps.SessionService,
		caps.Tokens,
		caps.SignupService,
		caps.LoginPhoneOTPSender,
		caps.PhoneLinkOTPSender,
		caps.LoginIdentityLinking,
		caps.KeyPublishApp,
	)
	*registrations = append(*registrations, grpctransport.Registration{
		Module:      "authn",
		Description: "AuthService, AuthSignupService, AuthChallengeService, LoginIdentityService, JWKSService",
		Register:    service.Register,
	})
}
