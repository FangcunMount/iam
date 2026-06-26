package idp

import (
	"time"

	wechatappDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
)

type idpDomainServices struct {
	wechatAppCreator  wechatappDomain.Creator
	credentialRotater wechatappDomain.CredentialRotater
	accessTokenCacher wechatappDomain.AccessTokenCacher
	appTokenProvider  wechatappDomain.AppTokenProvider
}

func (m *IDPModule) initializeDomain() (*idpDomainServices, error) {
	wechatAppCreator := wechatappDomain.NewCreator(m.wechatAppRepo)

	credentialRotater := wechatappDomain.NewCredentialRotater(
		m.secretVault,
		time.Now,
	)

	appTokenProvider := &appTokenProviderAdapter{
		tokenProvider: m.wechatTokenProvider,
		secretVault:   m.secretVault,
	}

	accessTokenCacher := wechatappDomain.NewAccessTokenCacher(
		m.accessTokenCache,
		appTokenProvider,
	)

	return &idpDomainServices{
		wechatAppCreator:  wechatAppCreator,
		credentialRotater: credentialRotater,
		accessTokenCacher: accessTokenCacher,
		appTokenProvider:  appTokenProvider,
	}, nil
}
