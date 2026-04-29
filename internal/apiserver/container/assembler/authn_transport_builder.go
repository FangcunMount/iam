package assembler

import (
	authngrpc "github.com/FangcunMount/iam/internal/apiserver/interface/authn/grpc"
	authhandler "github.com/FangcunMount/iam/internal/apiserver/interface/authn/restful/handler"
)

func (m *AuthnModule) initializeInterface() {
	m.AccountHandler = authhandler.NewAccountHandler(
		m.AccountService,
		m.RegisterService,
	)

	m.AuthHandler = authhandler.NewAuthHandler(
		m.LoginService,
		m.TokenService,
		m.LoginPreparationService,
	)

	m.JWKSHandler = authhandler.NewJWKSHandler(
		m.KeyManagementApp,
		m.KeyPublishApp,
	)
	m.SessionAdminHandler = authhandler.NewSessionAdminHandler(m.SessionService)

	m.GRPCService = authngrpc.NewService(
		m.TokenService,
		m.RegisterService,
		m.KeyPublishApp,
	)
}
