package idp

import (
	externalidentity "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/wechatapp"
)

func (m *IDPModule) initializeApplication(domainServices *idpDomainServices, externalConfig externalidentity.Config) error {
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
	m.externalResolver = externalidentity.NewResolver(externalidentity.Dependencies{
		Apps:      m.wechatAppRepo,
		Vault:     m.secretVault,
		Exchanger: m.externalExchanger,
		Config:    externalConfig,
	})

	return nil
}
