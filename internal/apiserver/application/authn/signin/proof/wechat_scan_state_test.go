package proof

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	challengeApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/challenge"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin/method"
	challengeDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/challenge"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

func TestWechatScanBuilderConsumesOAuthStateOnAppIDMismatch(t *testing.T) {
	t.Parallel()

	repo := newWechatScanOAuthChallengeRepoStub()
	oauthStates := challengeApp.NewService(repo, challengeApp.SMSOTPDelivery{}, challengeApp.NewCreator(repo), challengeApp.NewVerifier(repo))
	created, err := oauthStates.StartWechatOpenLogin(context.Background(), challengeApp.StartWechatOpenLoginInput{
		AppID:       "wx-app",
		RedirectURI: "https://example.com/callback",
	})
	require.NoError(t, err)

	resolver := &wecomResolverStub{}
	builder := newWechatScanBuilder(resolver, oauthStates)
	_, err = builder.Build(context.Background(), method.WechatScanPayload{
		AppID: "wx-other-app",
		Code:  "wx-code",
		State: created.State,
	}, method.CommonPayload{})
	require.Error(t, err)
	require.Equal(t, code.ErrStateMismatch, perrors.ParseCoder(err).Code())
	require.Zero(t, resolver.calls, "invalid state must be consumed before provider exchange")

	_, err = builder.Build(context.Background(), method.WechatScanPayload{
		AppID: "wx-app",
		Code:  "wx-code",
		State: created.State,
	}, method.CommonPayload{})
	require.Error(t, err)
	require.Equal(t, code.ErrStateMismatch, perrors.ParseCoder(err).Code())
	require.Zero(t, resolver.calls, "reused state must not reach provider exchange")
}

type wechatScanOAuthChallengeRepoStub struct {
	items map[string]*challengeDomain.AuthChallenge
}

func newWechatScanOAuthChallengeRepoStub() *wechatScanOAuthChallengeRepoStub {
	return &wechatScanOAuthChallengeRepoStub{items: map[string]*challengeDomain.AuthChallenge{}}
}

func (s *wechatScanOAuthChallengeRepoStub) Create(_ context.Context, challenge *challengeDomain.AuthChallenge) error {
	s.items[challenge.ID] = challenge
	return nil
}

func (s *wechatScanOAuthChallengeRepoStub) Get(_ context.Context, id string) (*challengeDomain.AuthChallenge, error) {
	return s.items[id], nil
}

func (s *wechatScanOAuthChallengeRepoStub) ConsumeIfSecretMatches(_ context.Context, id string, _ []byte) (bool, error) {
	if _, ok := s.items[id]; !ok {
		return false, nil
	}
	delete(s.items, id)
	return true, nil
}

func (s *wechatScanOAuthChallengeRepoStub) RecordFailedAttemptIfCurrent(
	context.Context,
	string,
	[]byte,
	int,
) (bool, bool, error) {
	return true, false, nil
}

func (s *wechatScanOAuthChallengeRepoStub) Delete(_ context.Context, id string) error {
	delete(s.items, id)
	return nil
}
