package challenge

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIssueOAuthStateAndVerifyOnce(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	repo := newOAuthRepoStub()
	issued, err := IssueOAuthState(OAuthStateSpec{
		Scene:       "wechat_open_login",
		AppID:       "wx-app",
		RedirectURI: "https://example.com/callback",
		Nonce:       "nonce-1",
		State:       "state-1",
		TTL:         time.Minute,
		Now:         now,
	})
	require.NoError(t, err)
	require.NoError(t, repo.Create(context.Background(), issued.Challenge))

	verifier := NewOAuthStateVerifier(repo)
	verification, err := verifier.VerifyAndConsume(context.Background(), VerifyOAuthStateInput{
		Scene: "wechat_open_login",
		State: "state-1",
		Now:   now,
	})
	require.NoError(t, err)
	require.Equal(t, VerificationSuccess, verification.Result.Outcome)
	require.Equal(t, "wx-app", verification.Context.AppID)
	require.Equal(t, "nonce-1", verification.Context.Nonce)

	rejected, err := verifier.VerifyAndConsume(context.Background(), VerifyOAuthStateInput{
		Scene: "wechat_open_login",
		State: "state-1",
		Now:   now,
	})
	require.Error(t, err)
	require.Equal(t, VerificationRejected, rejected.Result.Outcome)
}

type oauthRepoStub struct {
	items map[string]*AuthChallenge
}

func newOAuthRepoStub() *oauthRepoStub {
	return &oauthRepoStub{items: map[string]*AuthChallenge{}}
}

func (s *oauthRepoStub) Create(_ context.Context, item *AuthChallenge) error {
	s.items[item.ID] = item
	return nil
}

func (s *oauthRepoStub) Get(_ context.Context, id string) (*AuthChallenge, error) {
	return s.items[id], nil
}

func (s *oauthRepoStub) ConsumeIfSecretMatches(_ context.Context, id string, expectedHash []byte) (bool, error) {
	item := s.items[id]
	if item == nil || string(item.SecretHash) != string(expectedHash) {
		return false, nil
	}
	delete(s.items, id)
	return true, nil
}

func (s *oauthRepoStub) RecordFailedAttemptIfCurrent(context.Context, string, []byte, int) (bool, bool, error) {
	return false, false, nil
}

func (s *oauthRepoStub) Delete(_ context.Context, id string) error {
	delete(s.items, id)
	return nil
}
