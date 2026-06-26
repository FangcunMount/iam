package idp

import "github.com/FangcunMount/iam/v2/internal/apiserver/application/idp/wechatapp"

func (m *IDPModule) initializeApplication(domainServices *idpDomainServices) error {
	m.WechatAppService = wechatapp.NewWechatAppApplicationService(
		m.wechatAppRepo,
		domainServices.wechatAppCreator,
		domainServices.credentialRotater,
	)

	m.WechatAppCredentialService = wechatapp.NewWechatAppCredentialApplicationService(
		m.wechatAppRepo,
		domainServices.credentialRotater,
	)

	m.WechatAppTokenService = wechatapp.NewWechatAppTokenApplicationService(
		m.wechatAppRepo,
		domainServices.accessTokenCacher,
		domainServices.appTokenProvider,
		m.accessTokenCache,
	)

	return nil
}
