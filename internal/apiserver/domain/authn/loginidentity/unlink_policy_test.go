package loginidentity

import (
	"testing"
	"time"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestUnlinkPolicySensitiveProvidersRequireRecentAuthentication(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-time.Minute)
	policy := DefaultUnlinkPolicy()

	for _, provider := range []Provider{ProviderUsername, ProviderPhone} {
		t.Run(string(provider), func(t *testing.T) {
			identity := &LoginIdentity{ID: meta.FromUint64(1), Provider: provider}
			require.Equal(t, UnlinkReauthRequired, policy.AssessRecentAuthentication(UnlinkReauthRequest{
				Identity: identity,
				Now:      now,
			}))
			require.Equal(t, UnlinkReauthOK, policy.AssessRecentAuthentication(UnlinkReauthRequest{
				Identity:        identity,
				AuthenticatedAt: &recent,
				Now:             now,
			}))
		})
	}
}

func TestUnlinkPolicyCurrentSessionIdentityRequiresRecentAuthentication(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	identityID := meta.FromUint64(1)
	policy := DefaultUnlinkPolicy()

	require.Equal(t, UnlinkReauthRequired, policy.AssessRecentAuthentication(UnlinkReauthRequest{
		Identity:               &LoginIdentity{ID: identityID, Provider: ProviderWechatMinip},
		CurrentLoginIdentityID: identityID,
		Now:                    now,
	}))
}

func TestUnlinkPolicyNonSensitiveIdentityDoesNotRequireRecentAuthentication(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	policy := DefaultUnlinkPolicy()

	require.Equal(t, UnlinkReauthOK, policy.AssessRecentAuthentication(UnlinkReauthRequest{
		Identity: &LoginIdentity{ID: meta.FromUint64(1), Provider: ProviderWechatMinip},
		Now:      now,
	}))
}

func TestUnlinkPolicyRejectsExpiredAuthentication(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	expired := now.Add(-11 * time.Minute)
	policy := DefaultUnlinkPolicy()

	require.Equal(t, UnlinkReauthRequired, policy.AssessRecentAuthentication(UnlinkReauthRequest{
		Identity:        &LoginIdentity{ID: meta.FromUint64(1), Provider: ProviderUsername},
		AuthenticatedAt: &expired,
		Now:             now,
	}))
}

func TestUnlinkPolicyRejectsFutureAuthenticationBeyondClockSkew(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	future := now.Add(2 * time.Minute)
	policy := DefaultUnlinkPolicy()

	require.Equal(t, UnlinkReauthRequired, policy.AssessRecentAuthentication(UnlinkReauthRequest{
		Identity:        &LoginIdentity{ID: meta.FromUint64(1), Provider: ProviderUsername},
		AuthenticatedAt: &future,
		Now:             now,
	}))
}

func TestUnlinkPolicyAcceptsAuthenticationWithinFutureClockSkew(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	withinSkew := now.Add(30 * time.Second)
	policy := DefaultUnlinkPolicy()

	require.Equal(t, UnlinkReauthOK, policy.AssessRecentAuthentication(UnlinkReauthRequest{
		Identity:        &LoginIdentity{ID: meta.FromUint64(1), Provider: ProviderUsername},
		AuthenticatedAt: &withinSkew,
		Now:             now,
	}))
}
