package externalidentity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewNormalizesAndDeduplicatesIdentifiers(t *testing.T) {
	verifiedAt := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	openID, err := NewIdentifier(IdentifierOpenID, " open-1 ")
	require.NoError(t, err)
	duplicate, err := NewIdentifier(IdentifierOpenID, "open-1")
	require.NoError(t, err)
	unionID, err := NewIdentifier(IdentifierUnionID, " union-1 ")
	require.NoError(t, err)

	identity, err := New(ProviderWechatMinip, " app-1 ", []Identifier{openID, duplicate, unionID}, verifiedAt)
	require.NoError(t, err)
	require.Equal(t, ProviderWechatMinip, identity.Provider())
	require.Equal(t, "app-1", identity.Realm())
	require.Equal(t, verifiedAt, identity.VerifiedAt())
	require.Len(t, identity.Identifiers(), 2)
	value, ok := identity.Identifier(IdentifierOpenID)
	require.True(t, ok)
	require.Equal(t, "open-1", value)
}

func TestNewEnforcesProviderIdentifierInvariants(t *testing.T) {
	verifiedAt := time.Now()
	userID, err := NewIdentifier(IdentifierUserID, "user-1")
	require.NoError(t, err)
	openUserID, err := NewIdentifier(IdentifierOpenUserID, "open-user-1")
	require.NoError(t, err)

	_, err = New(ProviderWechatOpen, "app-1", []Identifier{userID}, verifiedAt)
	require.Error(t, err)

	identity, err := New(ProviderWecom, "corp-1", []Identifier{openUserID}, verifiedAt)
	require.NoError(t, err)
	value, ok := identity.Identifier(IdentifierOpenUserID)
	require.True(t, ok)
	require.Equal(t, "open-user-1", value)
}

func TestNewRejectsMissingVerificationContext(t *testing.T) {
	openID, err := NewIdentifier(IdentifierOpenID, "open-1")
	require.NoError(t, err)

	_, err = New(ProviderWechatMinip, "", []Identifier{openID}, time.Now())
	require.Error(t, err)
	_, err = New(ProviderWechatMinip, "app-1", []Identifier{openID}, time.Time{})
	require.Error(t, err)
}
