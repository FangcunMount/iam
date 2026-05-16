package verifier

import (
	"testing"

	"github.com/FangcunMount/iam/v2/pkg/tenant"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/stretchr/testify/require"
)

func TestNormalizeTenantClaim(t *testing.T) {
	t.Parallel()

	require.Equal(t, tenant.DefaultID, normalizeTenantClaim(""))
	require.Equal(t, "fangcun", normalizeTenantClaim("fangcun"))
	require.Equal(t, tenant.DefaultID, normalizeTenantClaim("1"))
}

func TestApplyTenantAndOrg(t *testing.T) {
	t.Parallel()

	t.Run("new token domain and org", func(t *testing.T) {
		claims := &TokenClaims{}
		applyTenantAndOrg(claims, "fangcun", "42")
		require.Equal(t, "fangcun", claims.TenantDomain)
		require.Equal(t, "fangcun", claims.TenantID)
		require.Equal(t, "42", claims.OrgID)
	})

	t.Run("legacy numeric tenant does not infer org", func(t *testing.T) {
		claims := &TokenClaims{}
		applyTenantAndOrg(claims, "1", "")
		require.Equal(t, tenant.DefaultID, claims.TenantDomain)
		require.Equal(t, tenant.DefaultID, claims.TenantID)
		require.Empty(t, claims.OrgID)
	})
}

func TestExtractClaimsTenantDomainAndOrgID(t *testing.T) {
	t.Parallel()

	token := jwt.New()
	require.NoError(t, token.Set("tenant_id", "fangcun"))
	require.NoError(t, token.Set("org_id", "42"))

	claims := extractClaims(token)
	require.Equal(t, "fangcun", claims.TenantDomain)
	require.Equal(t, "fangcun", claims.TenantID)
	require.Equal(t, "42", claims.OrgID)
	require.Equal(t, "fangcun", claims.AuthorizationDomain())
	orgID, ok := claims.BusinessOrgID()
	require.True(t, ok)
	require.Equal(t, uint64(42), orgID)
}

func TestExtractClaimsLegacyNumericTenantDoesNotInferOrg(t *testing.T) {
	t.Parallel()

	token := jwt.New()
	require.NoError(t, token.Set("tenant_id", "1"))

	claims := extractClaims(token)
	require.Equal(t, tenant.DefaultID, claims.TenantDomain)
	require.Empty(t, claims.OrgID)
	_, ok := claims.BusinessOrgID()
	require.False(t, ok)
}

func TestApplyTenantAndOrgFromRemoteFields(t *testing.T) {
	t.Parallel()

	claims := &TokenClaims{UserID: "1001"}
	applyTenantAndOrg(claims, "fangcun", "42")
	require.Equal(t, "fangcun", claims.AuthorizationDomain())
	orgID, ok := claims.BusinessOrgID()
	require.True(t, ok)
	require.Equal(t, uint64(42), orgID)
}
