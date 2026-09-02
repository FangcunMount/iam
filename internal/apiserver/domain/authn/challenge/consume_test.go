package challenge

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAssessUsability(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	ch := &AuthChallenge{
		Type:      TypeSMSOTP,
		Scene:     "login",
		ExpiresAt: now.Add(time.Minute),
	}

	require.Equal(t, UsabilityNotFound, AssessUsability(nil, now, TypeSMSOTP, "login"))
	require.Equal(t, UsabilityWrongType, AssessUsability(ch, now, TypeOAuthState, "login"))
	require.Equal(t, UsabilityWrongScene, AssessUsability(ch, now, TypeSMSOTP, "link_phone"))
	require.Equal(t, UsabilityOK, AssessUsability(ch, now, TypeSMSOTP, "login"))

	ch.ConsumedAt = &now
	require.Equal(t, UsabilityConsumed, AssessUsability(ch, now, TypeSMSOTP, "login"))

	ch.ConsumedAt = nil
	ch.ExpiresAt = now.Add(-time.Second)
	require.Equal(t, UsabilityExpired, AssessUsability(ch, now, TypeSMSOTP, "login"))
}

func TestConsumeOnceRejectsWhenSecretWasReplaced(t *testing.T) {
	t.Parallel()

	repo := newConsumeRepoStub()
	hash := []byte("current")
	repo.items["id-1"] = &AuthChallenge{SecretHash: hash}

	result, err := consumeOnce(context.Background(), repo, "id-1", []byte("other"))
	require.NoError(t, err)
	require.Equal(t, VerificationRejected, result.Outcome)
	require.NotNil(t, repo.items["id-1"])
}

func TestRecordFailedVerificationExhaustsChallenge(t *testing.T) {
	t.Parallel()

	repo := newConsumeRepoStub()
	hash := []byte("current")
	repo.items["id-1"] = &AuthChallenge{SecretHash: hash}

	result, err := recordFailedVerification(context.Background(), repo, "id-1", hash, 1)
	require.NoError(t, err)
	require.Equal(t, VerificationExhausted, result.Outcome)
	require.Nil(t, repo.items["id-1"])
}

type consumeRepoStub struct {
	items map[string]*AuthChallenge
}

func newConsumeRepoStub() *consumeRepoStub {
	return &consumeRepoStub{items: map[string]*AuthChallenge{}}
}

func (s *consumeRepoStub) Create(context.Context, *AuthChallenge) error { return nil }

func (s *consumeRepoStub) Get(_ context.Context, id string) (*AuthChallenge, error) {
	return s.items[id], nil
}

func (s *consumeRepoStub) ConsumeIfSecretMatches(_ context.Context, id string, expectedHash []byte) (bool, error) {
	item := s.items[id]
	if item == nil || !secretHashMatches(item.SecretHash, expectedHash) {
		return false, nil
	}
	delete(s.items, id)
	return true, nil
}

func (s *consumeRepoStub) RecordFailedAttemptIfCurrent(
	_ context.Context,
	id string,
	currentSecretHash []byte,
	maxAttempts int,
) (bool, bool, error) {
	item := s.items[id]
	if item == nil || !secretHashMatches(item.SecretHash, currentSecretHash) {
		return false, false, nil
	}
	item.Attempts++
	if item.Attempts >= maxAttempts {
		delete(s.items, id)
		return true, true, nil
	}
	return true, false, nil
}

func (s *consumeRepoStub) Delete(_ context.Context, id string) error {
	delete(s.items, id)
	return nil
}
