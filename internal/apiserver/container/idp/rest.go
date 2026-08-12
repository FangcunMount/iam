package idp

import (
	resttransport "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest"
	idphandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/idp/handler"
)

// CollectREST wires IDP REST handlers when the module is available.
func CollectREST(available bool, mod *IDPModule, deps *resttransport.Deps) {
	if !available || mod == nil || deps == nil {
		return
	}
	caps := mod.ApplicationCapabilities()
	deps.IDP.WechatAppHandler = idphandler.NewWechatAppHandler(
		caps.WechatAppService,
		caps.WechatAppCredentialService,
		caps.WechatAppTokenService,
	)
}
