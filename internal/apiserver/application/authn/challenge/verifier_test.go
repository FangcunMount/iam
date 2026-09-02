package challenge

import (
	"context"
	"errors"
	"testing"
	"time"

	challengeDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/challenge"
	"github.com/stretchr/testify/require"
)

func TestVerifierRejectsExpiredChallenge(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	repo := newChallengeRepoStub()
	repo.items[challengeDomain.SMSOTPChallengeID(SceneLoginPhoneOTP, "+8613800138000")] = &challengeDomain.AuthChallenge{
		ID:         challengeDomain.SMSOTPChallengeID(SceneLoginPhoneOTP, "+8613800138000"),
		Type:       challengeDomain.TypeSMSOTP,
		Scene:      SceneLoginPhoneOTP,
		Target:     "+8613800138000",
		SecretHash: challengeDomain.SMSOTPSecretHash(SceneLoginPhoneOTP, "+8613800138000", "1234"),
		ExpiresAt:  now.Add(-time.Minute),
		CreatedAt:  now.Add(-2 * time.Minute),
	}

	ok, err := NewVerifier(repo).VerifyAndConsume(context.Background(), SceneLoginPhoneOTP, "13800138000", "1234")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestVerifierReturnsInfrastructureError(t *testing.T) {
	t.Parallel()

	repo := newChallengeRepoStub()
	repo.consumeErr = errors.New("redis unavailable")
	repo.items[challengeDomain.SMSOTPChallengeID(SceneLoginPhoneOTP, "+8613800138000")] = &challengeDomain.AuthChallenge{
		ID:         challengeDomain.SMSOTPChallengeID(SceneLoginPhoneOTP, "+8613800138000"),
		Type:       challengeDomain.TypeSMSOTP,
		Scene:      SceneLoginPhoneOTP,
		Target:     "+8613800138000",
		SecretHash: challengeDomain.SMSOTPSecretHash(SceneLoginPhoneOTP, "+8613800138000", "1234"),
		ExpiresAt:  time.Now().Add(time.Minute),
		CreatedAt:  time.Now(),
	}

	ok, err := NewVerifier(repo).VerifyAndConsume(context.Background(), SceneLoginPhoneOTP, "13800138000", "1234")
	require.Error(t, err)
	require.False(t, ok)
}
