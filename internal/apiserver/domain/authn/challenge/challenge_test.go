package challenge_test

import (
	"testing"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/challenge"
	"github.com/stretchr/testify/require"
)

func TestAuthChallengeExpiry(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	c := &challenge.AuthChallenge{
		ID:         "challenge-1",
		Type:       challenge.TypeSMSOTP,
		Scene:      "login",
		Target:     "+8613800000000",
		SecretHash: []byte("hashed-secret"),
		ExpiresAt:  now.Add(time.Minute),
		CreatedAt:  now,
	}

	require.False(t, c.IsExpired(now))
	require.True(t, c.IsExpired(c.ExpiresAt), "expiry boundary is closed")
}

func TestNilChallengeIsUnavailable(t *testing.T) {
	var c *challenge.AuthChallenge
	require.True(t, c.IsExpired(time.Now()))
}

func TestChallengeTypesAreStableProtocolValues(t *testing.T) {
	require.Equal(t, challenge.ChallengeType("sms_otp"), challenge.TypeSMSOTP)
	require.Equal(t, challenge.ChallengeType("email_otp"), challenge.TypeEmailOTP)
	require.Equal(t, challenge.ChallengeType("oauth_state"), challenge.TypeOAuthState)
	require.Equal(t, challenge.ChallengeType("login_code"), challenge.TypeLoginCode)
}
