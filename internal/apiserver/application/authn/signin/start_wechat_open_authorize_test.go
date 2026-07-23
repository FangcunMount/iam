package signin_test

import (
	"context"
	"strings"
	"testing"
	"time"

	challengeApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/challenge"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin"
	challengeDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/challenge"
	"github.com/stretchr/testify/require"
)

func TestStartWechatOpenAuthorizeReturnsStateAndAuthorizeURL(t *testing.T) {
	t.Parallel()

	repo := newAuthorizeChallengeRepoStub()
	service := challengeApp.NewService(
		repo,
		challengeApp.SMSOTPDelivery{},
		challengeApp.NewCreator(repo),
		challengeApp.NewVerifier(repo),
	)
	useCase := signin.NewStartWechatOpenAuthorize(service, stubAuthorizeURLBuilder{})

	result, err := useCase.Execute(context.Background(), signin.StartWechatOpenAuthorizeInput{
		AppID:       "wx-app",
		RedirectURI: "https://example.com/callback",
		Nonce:       "nonce-1",
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.State)
	require.Equal(t, "nonce-1", result.Nonce)
	require.True(t, strings.Contains(result.AuthorizeURL, result.State))
	require.True(t, result.ExpiresAt.After(time.Now()))
}

type stubAuthorizeURLBuilder struct{}

func (stubAuthorizeURLBuilder) BuildWebAppQRConnectURL(appID, redirectURI, state string) (string, error) {
	return "https://open.weixin.qq.com/connect/qrconnect?appid=" + appID + "&state=" + state + "&redirect_uri=" + redirectURI, nil
}

type authorizeChallengeRepoStub struct {
	items map[string]*challengeDomain.AuthChallenge
}

func newAuthorizeChallengeRepoStub() *authorizeChallengeRepoStub {
	return &authorizeChallengeRepoStub{items: map[string]*challengeDomain.AuthChallenge{}}
}

func (s *authorizeChallengeRepoStub) Create(_ context.Context, challenge *challengeDomain.AuthChallenge) error {
	s.items[challenge.ID] = challenge
	return nil
}

func (s *authorizeChallengeRepoStub) Get(_ context.Context, id string) (*challengeDomain.AuthChallenge, error) {
	return s.items[id], nil
}

func (s *authorizeChallengeRepoStub) ConsumeIfSecretMatches(_ context.Context, id string, _ []byte) (bool, error) {
	if _, ok := s.items[id]; !ok {
		return false, nil
	}
	delete(s.items, id)
	return true, nil
}

func (s *authorizeChallengeRepoStub) Delete(_ context.Context, id string) error {
	delete(s.items, id)
	return nil
}
