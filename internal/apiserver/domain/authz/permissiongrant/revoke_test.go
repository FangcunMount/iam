package permissiongrant_test

import (
	"testing"

	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/stretchr/testify/require"
)

func TestRevokeOutcomeAppliesVersionChangeOnlyForFreshRevoke(t *testing.T) {
	require.True(t, domain.RevokeOutcomeRevoked.AppliesVersionChange())
	require.False(t, domain.RevokeOutcomeAlreadyRevoked.AppliesVersionChange())
	require.False(t, domain.RevokeOutcomeNotFound.AppliesVersionChange())
}

func TestRevokeOutcomeSuccessTreatsAlreadyRevokedAsIdempotent(t *testing.T) {
	require.True(t, domain.RevokeOutcomeRevoked.IsSuccess())
	require.True(t, domain.RevokeOutcomeAlreadyRevoked.IsSuccess())
	require.False(t, domain.RevokeOutcomeNotFound.IsSuccess())
}
