package authn

import (
	resttransport "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest"
	authnhandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authn/handler"
)

// CollectREST wires authn REST handlers when the module is available.
func CollectREST(available bool, mod *AuthnModule, moduleName string, deps *resttransport.Deps) {
	if !available || mod == nil || deps == nil {
		return
	}
	caps := mod.ApplicationCapabilities()
	deps.ModuleStatus.Authn = deps.ModuleStatus.Modules[moduleName].Available
	deps.Authn.AuthHandler = authnhandler.NewAuthHandler(caps.SessionService, caps.TokenService, caps.LoginPhoneOTPSender)
	deps.Authn.OnboardingHandler = authnhandler.NewOnboardingHandler(caps.SignupService)
	deps.Authn.LoginIdentityHandler = authnhandler.NewLoginIdentityHandler(
		caps.LoginIdentityLinking,
		caps.PhoneLinkOTPSender,
		caps.StartWechatOpenLinkAuthorize,
		caps.CompleteWechatOpenLink,
		authnhandler.WechatOpenLinkConfig{
			AppID:       caps.WechatOpen.AppID,
			RedirectURI: caps.WechatOpen.LinkRedirectURI,
		},
	)
	deps.Authn.WechatOpenLoginHandler = authnhandler.NewWechatOpenLoginAuthorizeHandler(
		caps.StartWechatOpenAuthorize,
		caps.WechatOpen.AppID,
		caps.WechatOpen.LoginRedirectURI,
	)
	deps.Authn.SignupService = caps.SignupService
	deps.Authn.LoginIdentityLinking = caps.LoginIdentityLinking
	deps.Authn.JWKSHandler = authnhandler.NewJWKSHandler(caps.KeyManagementApp, caps.KeyLifecycleApp, caps.KeyPublishApp)
	deps.Authn.SessionAdminHandler = authnhandler.NewSessionAdminHandler(caps.SessionRevoker)
	deps.Authn.TokenService = caps.TokenService
	deps.ModuleStatus.AuthEnabled = caps.TokenService != nil
}
