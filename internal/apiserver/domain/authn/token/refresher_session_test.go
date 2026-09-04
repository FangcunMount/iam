package token

import (
	"testing"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestPrincipalFromSessionPrefersSessionContext(t *testing.T) {
	s := &refresher{legacyContextDecoder: normalizeLegacyContextDecoder(nil)}
	authenticatedAt := time.Unix(1700000100, 0).UTC()
	sess := sessiondomain.NewWithContexts(
		"sid", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3),
		authentication.RestoreAuthenticationContext(authentication.MethodPassword, "global", []authentication.AMR{authentication.AMRPassword}, authenticatedAt),
		authentication.TokenContext{TenantDomain: "fangcun"}, time.Now().Add(time.Hour),
	)

	refresh := NewRefreshToken("rid", "rval", "sid", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3), []string{"otp"}, map[string]string{"tenant_domain": "legacy"}, time.Hour)
	refresh.AuthMethod = "phone_otp"
	refresh.Realm = "legacy-realm"

	principal := s.principalFromSession(sess, refresh)
	require.Equal(t, "password", string(principal.AuthContext.Method))
	require.Equal(t, "global", principal.AuthContext.Realm)
	require.Equal(t, []string{"pwd"}, principal.AuthContext.AMRStrings())
	require.Equal(t, "fangcun", principal.TokenContext.TenantDomain)
	require.Equal(t, authenticatedAt, principal.AuthContext.AuthenticatedAt)
}

func TestPrincipalFromSessionFallsBackToRefreshToken(t *testing.T) {
	s := &refresher{legacyContextDecoder: normalizeLegacyContextDecoder(nil)}
	sess := &sessiondomain.Session{
		SessionID: "sid", UserID: meta.FromUint64(1), LoginIdentityID: meta.FromUint64(2), TenantID: meta.FromUint64(3),
		Status: sessiondomain.StatusActive, CreatedAt: time.Now().Add(-time.Hour), ExpiresAt: time.Now().Add(time.Hour),
	}
	refresh := NewRefreshToken("rid", "rval", "sid", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3), []string{"otp"}, map[string]string{"tenant_domain": "legacy"}, time.Hour)
	refresh.AuthMethod = "phone_otp"
	refresh.Realm = "legacy-realm"

	principal := s.principalFromSession(sess, refresh)
	require.Equal(t, "phone_otp", string(principal.AuthContext.Method))
	require.Equal(t, "legacy-realm", principal.AuthContext.Realm)
	require.Equal(t, []string{"otp"}, principal.AuthContext.AMRStrings())
	require.Equal(t, "legacy", principal.TokenContext.TenantDomain)
	require.Equal(t, sess.CreatedAt.UTC(), principal.AuthContext.AuthenticatedAt)
}

func TestPrincipalFromSessionDoesNotInventMissingHistoricalAuthTime(t *testing.T) {
	s := &refresher{legacyContextDecoder: normalizeLegacyContextDecoder(nil)}
	sess := &sessiondomain.Session{
		SessionID: "sid", UserID: meta.FromUint64(1), LoginIdentityID: meta.FromUint64(2),
		TenantID: meta.FromUint64(3), Status: sessiondomain.StatusActive, ExpiresAt: time.Now().Add(time.Hour),
	}
	refresh := NewRefreshToken("rid", "rval", "sid", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3), []string{"pwd"}, nil, time.Hour)
	refresh.AuthMethod = "password"
	refresh.Realm = "global"

	principal := s.principalFromSession(sess, refresh)
	require.True(t, principal.AuthContext.AuthenticatedAt.IsZero())
}

func TestAccessTokenSubjectKeepsAuthContextAuthenticatedAt(t *testing.T) {
	authenticatedAt := time.Unix(1700000200, 0).UTC()
	principal := &authentication.Principal{
		UserID: meta.FromUint64(10), LoginIdentityID: meta.FromUint64(20),
		TokenContext: authentication.TokenContext{TenantDomain: "fangcun"},
	}
	principal.ApplyAuthContext(authentication.NewAuthenticationContext(authentication.MethodPassword, "global", []authentication.AMR{authentication.AMRPassword}, authenticatedAt))
	sess := &sessiondomain.Session{
		SessionID: "sid-1", UserID: meta.FromUint64(10), LoginIdentityID: meta.FromUint64(20),
		AuthContext: authentication.RestoreAuthenticationContext(authentication.MethodPassword, "global", []authentication.AMR{authentication.AMRPassword}, time.Unix(1, 0).UTC()),
	}
	got := accessTokenSubjectFromAuth(principal, sess)
	require.Equal(t, authenticatedAt, got.AuthenticatedAt)
}
