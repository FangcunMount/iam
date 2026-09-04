package authentication

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuthenticationContextCloneIsDefensive(t *testing.T) {
	ctx := NewAuthenticationContext(MethodPassword, "global", []AMR{AMRPassword}, time.Unix(1700000000, 0).UTC())
	cloned := ctx.Clone()
	cloned.AMR[0] = AMR("mutated")
	require.Equal(t, AMRPassword, ctx.AMR[0])
	require.Equal(t, time.Unix(1700000000, 0).UTC(), cloned.AuthenticatedAt)
}

func TestApplyAuthContextUsesSingleAuthority(t *testing.T) {
	principal := &Principal{}
	principal.ApplyAuthContext(NewAuthenticationContext(MethodPhoneOTP, "global", []AMR{AMROTP}, time.Unix(1700000001, 0).UTC()))
	require.Equal(t, MethodPhoneOTP, principal.AuthContext.Method)
	require.Equal(t, "phone_otp", string(principal.AuthContext.Method))
	require.Equal(t, "global", principal.AuthContext.Realm)
	require.Equal(t, []string{"otp"}, principal.AuthContext.AMRStrings())
	require.Equal(t, time.Unix(1700000001, 0).UTC(), principal.AuthContext.AuthenticatedAt)
}

func TestRealmIsIdentityNamespaceNotAuthorizationDomain(t *testing.T) {
	ctx := NewAuthenticationContext(MethodWechatMinip, "wx-app-id", []AMR{AMRWx}, time.Now().UTC())
	require.Equal(t, "wx-app-id", ctx.Realm)
	require.NotEqual(t, "fangcun", ctx.Realm)
}

func TestRestoreAuthenticationContextPreservesUnknownAuthenticatedAt(t *testing.T) {
	ctx := RestoreAuthenticationContext(MethodPassword, "global", []AMR{AMRPassword}, time.Time{})
	require.True(t, ctx.AuthenticatedAt.IsZero())
}
