package challenge_test

import (
	"testing"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/challenge"
	"github.com/stretchr/testify/require"
)

func TestSMSOTPIdentityFormatsAreStable(t *testing.T) {
	t.Parallel()

	require.Equal(t, "sms_otp:login:+8613800138000", challenge.SMSOTPChallengeID("login", "+8613800138000"))
	hash := challenge.SMSOTPSecretHash("login", "+8613800138000", "1234")
	require.Len(t, hash, 32)
	require.Equal(t, hash, challenge.SMSOTPSecretHash("login", "+8613800138000", "1234"))
}

func TestOAuthStateIdentityFormatsAreStable(t *testing.T) {
	t.Parallel()

	require.Equal(t, "oauth_state:wechat_open_login:state-token", challenge.OAuthStateChallengeID("wechat_open_login", "state-token"))
	hash := challenge.OAuthStateSecretHash("state-token")
	require.Len(t, hash, 32)
	require.Equal(t, hash, challenge.OAuthStateSecretHash("state-token"))
}
