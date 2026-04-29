package assembler

import (
	idpGrpc "github.com/FangcunMount/iam/internal/apiserver/interface/idp/grpc"
	"github.com/FangcunMount/iam/internal/apiserver/interface/idp/restful/handler"
)

func (m *IDPModule) initializeInterface() error {
	m.WechatAppHandler = handler.NewWechatAppHandler(
		m.WechatAppService,
		m.WechatAppCredentialService,
		m.WechatAppTokenService,
	)

	m.GRPCService = idpGrpc.NewService(
		m.WechatAppService,
		m.wechatAppRepo,
		m.secretVault,
	)

	return nil
}
