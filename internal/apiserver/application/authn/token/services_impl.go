package token

// ============= TokenApplicationService 实现 =============

type tokenApplicationService struct {
	serviceTokenIssuer ServiceTokenIssuer
	accessRevoker      AccessRevoker
	tokenRefresher     Refresher
	tokenVerifier      Verifier
}

var _ TokenApplicationService = (*tokenApplicationService)(nil)

func NewTokenApplicationService(
	tokenIssuer Issuer,
	tokenRefresher Refresher,
	tokenVerifier Verifier,
) TokenApplicationService {
	return &tokenApplicationService{
		serviceTokenIssuer: tokenIssuer,
		accessRevoker:      tokenIssuer,
		tokenRefresher:     tokenRefresher,
		tokenVerifier:      tokenVerifier,
	}
}
