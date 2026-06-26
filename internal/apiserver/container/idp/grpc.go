package idp

import (
	grpctransport "github.com/FangcunMount/iam/v2/internal/apiserver/transport/grpc"
	idpgrpc "github.com/FangcunMount/iam/v2/internal/apiserver/transport/grpc/service/idp"
)

// CollectGRPC appends IDP gRPC registration when the module is available.
func CollectGRPC(available bool, mod *IDPModule, registrations *[]grpctransport.Registration) {
	if !available || mod == nil || registrations == nil {
		return
	}
	caps := mod.ApplicationCapabilities()
	service := idpgrpc.NewService(caps.WechatAppService, caps.WechatAppTokenService, caps.WechatAppRepository, caps.SecretVault)
	*registrations = append(*registrations, grpctransport.Registration{
		Module:      "idp",
		Description: "IDPService",
		Register:    service.Register,
	})
}
