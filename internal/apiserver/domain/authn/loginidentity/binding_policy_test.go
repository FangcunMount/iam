package loginidentity

import (
	"testing"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestAssessBinding(t *testing.T) {
	t.Parallel()
	userID := meta.FromUint64(100)
	otherUser := meta.FromUint64(200)

	tests := []struct {
		name     string
		existing *LoginIdentity
		want     BindingDecision
	}{
		{name: "create when missing", existing: nil, want: BindingCreate},
		{
			name:     "reuse same user active",
			existing: &LoginIdentity{UserID: userID, Status: StatusActive},
			want:     BindingReuse,
		},
		{
			name:     "conflict other user",
			existing: &LoginIdentity{UserID: otherUser, Status: StatusActive},
			want:     BindingConflictOtherUser,
		},
		{
			name:     "reject inactive same user",
			existing: &LoginIdentity{UserID: userID, Status: StatusDeleted},
			want:     BindingInactiveSameUser,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, AssessBinding(BindingRequest{
				RequestUserID: userID,
				Existing:      tt.existing,
			}))
		})
	}
}

func TestAssessGlobalIdentifierAvailability(t *testing.T) {
	t.Parallel()
	userID := meta.FromUint64(100)
	require.Equal(t, GlobalIdentifierAvailable, AssessGlobalIdentifierAvailability(userID, nil))
	require.Equal(t, GlobalIdentifierAvailable, AssessGlobalIdentifierAvailability(userID, &LoginIdentity{UserID: userID}))
	require.Equal(t, GlobalIdentifierOwnedByOtherUser, AssessGlobalIdentifierAvailability(userID, &LoginIdentity{UserID: meta.FromUint64(200)}))
}

func TestAssessCanonicalClaim(t *testing.T) {
	t.Parallel()
	userID := meta.FromUint64(100)
	require.Equal(t, CanonicalClaimStoreOnNewRow, AssessCanonicalClaim(userID, nil))
	require.Equal(t, CanonicalClaimConflictOtherUser, AssessCanonicalClaim(userID, &LoginIdentity{UserID: meta.FromUint64(200), Status: StatusActive}))
	require.Equal(t, CanonicalClaimKeepExistingAnchor, AssessCanonicalClaim(userID, &LoginIdentity{UserID: userID, Status: StatusActive}))
	require.Equal(t, CanonicalClaimTransferFromInactiveAnchor, AssessCanonicalClaim(userID, &LoginIdentity{UserID: userID, Status: StatusDeleted}))
}

func TestSelectCanonicalReplacement(t *testing.T) {
	t.Parallel()
	target := &LoginIdentity{
		ID:               meta.FromUint64(1),
		Provider:         ProviderWechatMinip,
		GlobalIdentifier: "union-1",
		Status:           StatusActive,
	}
	replacement := &LoginIdentity{
		ID:       meta.FromUint64(2),
		Provider: ProviderWechatMinip,
		Status:   StatusActive,
	}
	otherProvider := &LoginIdentity{
		ID:       meta.FromUint64(3),
		Provider: ProviderWecom,
		Status:   StatusActive,
	}
	inactive := &LoginIdentity{
		ID:       meta.FromUint64(4),
		Provider: ProviderWechatMinip,
		Status:   StatusDeleted,
	}

	require.Nil(t, SelectCanonicalReplacement(nil, nil))
	require.Nil(t, SelectCanonicalReplacement(&LoginIdentity{ID: meta.FromUint64(9)}, nil))
	require.Nil(t, SelectCanonicalReplacement(target, []*LoginIdentity{otherProvider, inactive}))

	got := SelectCanonicalReplacement(target, []*LoginIdentity{otherProvider, replacement, inactive})
	require.NotNil(t, got)
	require.Equal(t, target.ID, got.TargetID)
	require.Equal(t, replacement.ID, got.ReplacementID)
	require.Equal(t, "union-1", got.GlobalIdentifier)
}
