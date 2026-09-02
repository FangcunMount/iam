package challenge_test

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/challenge"
	"github.com/stretchr/testify/require"
)

func TestIssueSMSOTPAndVerifyOnce(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	repo := newChallengeRepoStub()
	issued, err := challenge.IssueSMSOTP(challenge.SMSOTPSpec{
		Scene:     "login",
		PhoneE164: "+8613800138000",
		OTP:       "1234",
		TTL:       time.Minute,
		Now:       now,
	})
	require.NoError(t, err)
	require.NoError(t, repo.Create(context.Background(), issued.Challenge))

	verifier := challenge.NewSMSOTPVerifier(repo)
	ok, err := verifier.VerifyAndConsume(context.Background(), challenge.VerifySMSOTPInput{
		Scene:     "login",
		PhoneE164: "+8613800138000",
		OTP:       "1234",
		Now:       now,
	})
	require.NoError(t, err)
	require.Equal(t, challenge.VerificationSuccess, ok.Outcome)

	rejected, err := verifier.VerifyAndConsume(context.Background(), challenge.VerifySMSOTPInput{
		Scene:     "login",
		PhoneE164: "+8613800138000",
		OTP:       "1234",
		Now:       now,
	})
	require.NoError(t, err)
	require.Equal(t, challenge.VerificationRejected, rejected.Outcome)
}

func TestSMSOTPVerifierRejectsExpiredChallenge(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	repo := newChallengeRepoStub()
	repo.items[challenge.SMSOTPChallengeID("login", "+8613800138000")] = &challenge.AuthChallenge{
		ID:         challenge.SMSOTPChallengeID("login", "+8613800138000"),
		Type:       challenge.TypeSMSOTP,
		Scene:      "login",
		Target:     "+8613800138000",
		SecretHash: challenge.SMSOTPSecretHash("login", "+8613800138000", "1234"),
		ExpiresAt:  now.Add(-time.Minute),
		CreatedAt:  now.Add(-2 * time.Minute),
	}

	verifier := challenge.NewSMSOTPVerifier(repo)
	result, err := verifier.VerifyAndConsume(context.Background(), challenge.VerifySMSOTPInput{
		Scene:     "login",
		PhoneE164: "+8613800138000",
		OTP:       "1234",
		Now:       now,
	})
	require.NoError(t, err)
	require.Equal(t, challenge.VerificationRejected, result.Outcome)
}

func TestSMSOTPVerifierExhaustsAfterMaxAttempts(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	repo := newChallengeRepoStub()
	issued, err := challenge.IssueSMSOTP(challenge.SMSOTPSpec{
		Scene:     "login",
		PhoneE164: "+8613800138000",
		OTP:       "1234",
		TTL:       time.Minute,
		Now:       now,
	})
	require.NoError(t, err)
	require.NoError(t, repo.Create(context.Background(), issued.Challenge))

	verifier := challenge.NewSMSOTPVerifier(repo, 2)
	result, verifyErr := verifier.VerifyAndConsume(context.Background(), challenge.VerifySMSOTPInput{
		Scene:     "login",
		PhoneE164: "+8613800138000",
		OTP:       "0000",
		Now:       now,
	})
	require.NoError(t, verifyErr)
	require.Equal(t, challenge.VerificationRejected, result.Outcome)

	result, err = verifier.VerifyAndConsume(context.Background(), challenge.VerifySMSOTPInput{
		Scene:     "login",
		PhoneE164: "+8613800138000",
		OTP:       "0000",
		Now:       now,
	})
	require.NoError(t, err)
	require.Equal(t, challenge.VerificationExhausted, result.Outcome)

	result, err = verifier.VerifyAndConsume(context.Background(), challenge.VerifySMSOTPInput{
		Scene:     "login",
		PhoneE164: "+8613800138000",
		OTP:       "0000",
		Now:       now,
	})
	require.NoError(t, err)
	require.Equal(t, challenge.VerificationRejected, result.Outcome)
}

type challengeRepoStub struct {
	items    map[string]*challenge.AuthChallenge
	attempts map[string]int
}

func newChallengeRepoStub() *challengeRepoStub {
	return &challengeRepoStub{
		items:    map[string]*challenge.AuthChallenge{},
		attempts: map[string]int{},
	}
}

func (s *challengeRepoStub) Create(_ context.Context, item *challenge.AuthChallenge) error {
	s.items[item.ID] = item
	return nil
}

func (s *challengeRepoStub) Get(_ context.Context, id string) (*challenge.AuthChallenge, error) {
	return s.items[id], nil
}

func (s *challengeRepoStub) ConsumeIfSecretMatches(_ context.Context, id string, expectedHash []byte) (bool, error) {
	item := s.items[id]
	if item == nil {
		return false, nil
	}
	if string(item.SecretHash) != string(expectedHash) {
		return false, nil
	}
	delete(s.items, id)
	return true, nil
}

func (s *challengeRepoStub) RecordFailedAttemptIfCurrent(
	_ context.Context,
	id string,
	currentSecretHash []byte,
	maxAttempts int,
) (bool, bool, error) {
	item := s.items[id]
	if item == nil || string(item.SecretHash) != string(currentSecretHash) {
		return false, false, nil
	}
	s.attempts[id]++
	if s.attempts[id] >= maxAttempts {
		delete(s.items, id)
		delete(s.attempts, id)
		return true, true, nil
	}
	return true, false, nil
}

func (s *challengeRepoStub) Delete(_ context.Context, id string) error {
	delete(s.items, id)
	return nil
}
