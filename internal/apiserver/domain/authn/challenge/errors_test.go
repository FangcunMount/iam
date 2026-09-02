package challenge_test

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/challenge"
	"github.com/stretchr/testify/require"
)

func TestSMSOTPVerifierNilRepositoryReturnsInfrastructureError(t *testing.T) {
	t.Parallel()

	verifier := challenge.NewSMSOTPVerifier(nil)
	require.Nil(t, verifier)
}

func TestOAuthStateVerifierNilRepositoryReturnsInfrastructureError(t *testing.T) {
	t.Parallel()

	verifier := challenge.NewOAuthStateVerifier(nil)
	require.Nil(t, verifier)
}

func TestOAuthStateVerifierRejectsWhenRepositoryMissingAtRuntime(t *testing.T) {
	t.Parallel()

	var verifier *challenge.OAuthStateVerifier
	verification, err := verifier.VerifyAndConsume(context.Background(), challenge.VerifyOAuthStateInput{
		Scene: "wechat_open_login",
		State: "state-1",
		Now:   time.Now(),
	})
	require.ErrorIs(t, err, challenge.ErrRepositoryNotConfigured)
	require.Equal(t, challenge.VerificationInfrastructureError, verification.Result.Outcome)
}
