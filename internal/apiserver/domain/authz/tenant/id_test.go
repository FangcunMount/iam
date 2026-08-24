package tenant_test

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

func TestIDNormalizesNonEmptyTenant(t *testing.T) {
	id, err := tenant.NewID(" fangcun ")
	require.NoError(t, err)
	require.Equal(t, "fangcun", id.String())
	require.False(t, id.IsZero())
}

func TestIDRejectsBlankTenant(t *testing.T) {
	_, err := tenant.NewID(" \t ")
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
	require.True(t, tenant.ID("  ").IsZero())
}
