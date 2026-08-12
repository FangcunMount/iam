package jwt

import (
	"testing"

	"github.com/FangcunMount/iam/v3/pkg/tenant"
	"github.com/stretchr/testify/require"
)

func TestParseTenantIDClaim(t *testing.T) {
	t.Parallel()

	domain, legacy := parseTenantIDClaim("")
	require.Equal(t, tenant.DefaultID, domain)
	require.Equal(t, uint64(0), legacy)

	domain, legacy = parseTenantIDClaim("fangcun")
	require.Equal(t, "fangcun", domain)
	require.Equal(t, uint64(0), legacy)

	domain, legacy = parseTenantIDClaim("1")
	require.Equal(t, tenant.DefaultID, domain)
	require.Equal(t, uint64(1), legacy)
}
