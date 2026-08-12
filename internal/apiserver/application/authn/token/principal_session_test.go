package token

import (
	"testing"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestValidatePrincipalSessionAlignmentRejectsMismatchedUser(t *testing.T) {
	principal := &authentication.Principal{UserID: meta.FromUint64(1), LoginIdentityID: meta.FromUint64(2)}
	sess := &sessiondomain.Session{UserID: meta.FromUint64(9), LoginIdentityID: meta.FromUint64(2), SessionID: "sid"}
	require.Error(t, validatePrincipalSessionAlignment(principal, sess))
}

func TestResolveMintTenantIDPrefersSession(t *testing.T) {
	principal := &authentication.Principal{TenantID: meta.FromUint64(1)}
	sess := &sessiondomain.Session{TenantID: meta.FromUint64(2)}
	require.Equal(t, meta.FromUint64(2), resolveMintTenantID(principal, sess))
}

func TestAccessTokenSubjectFromAuthUsesSessionSessionID(t *testing.T) {
	principal := &authentication.Principal{
		UserID:          meta.FromUint64(10),
		LoginIdentityID: meta.FromUint64(20),
		TenantID:        meta.FromUint64(1),
		AuthMethod:      "password",
		Realm:           "fangcun",
	}
	sess := &sessiondomain.Session{
		SessionID:       "sid-1",
		UserID:          meta.FromUint64(10),
		LoginIdentityID: meta.FromUint64(20),
		TenantID:        meta.FromUint64(2),
		AuthMethod:      "password",
		Realm:           "fangcun",
	}
	got := accessTokenSubjectFromAuth(principal, sess)
	require.Equal(t, "sid-1", got.SessionID)
	require.Equal(t, meta.FromUint64(2), got.TenantID)
}
