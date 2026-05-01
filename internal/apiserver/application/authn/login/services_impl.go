package login

import (
	"context"

	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	idpPort "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
)

type loginApplicationService struct {
	signIn  *SignIn
	signOut *SignOut
}

var _ LoginApplicationService = (*loginApplicationService)(nil)

func NewLoginApplicationService(
	tokenIssuer tokenapp.Issuer,
	tokenRefresher tokenapp.Refresher,
	authenticator *authentication.Authenticator,
	tokenVerifier tokenapp.Verifier,
	wechatAppQuerier idpPort.Repository,
	secretVault idpPort.SecretVault,
	wecomConfig WecomConfig,
) LoginApplicationService {
	adapterCatalog := newDefaultSignInAdapterCatalog(signInAdapterDeps{
		wechatAppQuerier: wechatAppQuerier,
		secretVault:      secretVault,
		wecomConfig:      wecomConfig,
		tokenVerifier:    tokenVerifier,
	})
	return &loginApplicationService{
		signIn: &SignIn{
			tokenIssuer:         tokenIssuer,
			methodSelector:      newDefaultMethodSelector(adapterCatalog),
			domainAuthenticator: authenticator,
			failureTranslator:   AuthFailureTranslator{},
		},
		signOut: &SignOut{
			tokenIssuer:    tokenIssuer,
			tokenRefresher: tokenRefresher,
		},
	}
}

func (s *loginApplicationService) Login(ctx context.Context, req LoginRequest) (*LoginResult, error) {
	if s.signIn == nil {
		s.signIn = &SignIn{}
	}
	return s.signIn.Execute(ctx, req)
}

func (s *loginApplicationService) Logout(ctx context.Context, req LogoutRequest) error {
	if s.signOut == nil {
		s.signOut = &SignOut{}
	}
	return s.signOut.Execute(ctx, req)
}
