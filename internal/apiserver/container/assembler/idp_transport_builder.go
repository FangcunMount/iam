package assembler

import (
	idpGrpc "github.com/FangcunMount/iam/internal/apiserver/transport/grpc/service/idp"
	"github.com/FangcunMount/iam/internal/apiserver/transport/rest/idp/handler"
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
