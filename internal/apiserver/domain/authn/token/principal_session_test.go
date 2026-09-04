package token

import (
	"testing"
	"time"

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
		UserID: meta.FromUint64(10), LoginIdentityID: meta.FromUint64(20), TenantID: meta.FromUint64(1),
		AuthContext:  authentication.RestoreAuthenticationContext(authentication.MethodPassword, "wx-app", []authentication.AMR{authentication.AMRPassword}, time.Time{}),
		TokenContext: authentication.TokenContext{TenantDomain: "fangcun", OrgID: meta.FromUint64(42)},
	}
	sess := &sessiondomain.Session{
		SessionID: "sid-1", UserID: meta.FromUint64(10), LoginIdentityID: meta.FromUint64(20),
		TenantID:    meta.FromUint64(2),
		AuthContext: authentication.RestoreAuthenticationContext(authentication.MethodPassword, "wx-app", []authentication.AMR{authentication.AMRPassword}, time.Time{}),
	}
	got := accessTokenSubjectFromAuth(principal, sess)
	require.Equal(t, "sid-1", got.SessionID)
	require.Equal(t, meta.FromUint64(2), got.TenantID)
	require.Equal(t, "fangcun", got.TenantDomain)
	require.Equal(t, "42", got.OrgID)
}

func TestAccessTokenSubjectFromAuthDoesNotUseRealmAsTenantDomain(t *testing.T) {
	principal := &authentication.Principal{
		UserID: meta.FromUint64(10), LoginIdentityID: meta.FromUint64(20),
		AuthContext: authentication.RestoreAuthenticationContext(authentication.MethodWechatMinip, "wx-app", nil, time.Time{}),
	}
	sess := &sessiondomain.Session{
		SessionID: "sid-1", UserID: meta.FromUint64(10), LoginIdentityID: meta.FromUint64(20),
		AuthContext: authentication.RestoreAuthenticationContext(authentication.MethodWechatMinip, "wx-app", nil, time.Time{}),
	}
	got := accessTokenSubjectFromAuth(principal, sess)
	require.Equal(t, "fangcun", got.TenantDomain)
}
