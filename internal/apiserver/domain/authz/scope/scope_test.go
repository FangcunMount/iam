package scope_test

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

func TestScopeDefaultsAndRoundTrips(t *testing.T) {
	defaultScope, err := scope.Normalize("", "")
	require.NoError(t, err)
	require.Equal(t, scope.KindAll, defaultScope.Kind)
	require.Equal(t, "all:*", defaultScope.String())

	origin, err := scope.New(scope.KindOrigin, "hospital-a")
	require.NoError(t, err)
	parsed, err := scope.Parse(origin.String())
	require.NoError(t, err)
	require.Equal(t, origin, parsed)

	require.Equal(t, scope.Default(), (scope.Scope{}).Normalized())
}

func TestScopeRejectsInvalidCombinations(t *testing.T) {
	for _, tc := range []struct {
		name  string
		kind  scope.Kind
		value string
	}{
		{name: "value without kind", value: "hospital-a"},
		{name: "all with concrete value", kind: scope.KindAll, value: "hospital-a"},
		{name: "origin without value", kind: scope.KindOrigin},
		{name: "origin wildcard", kind: scope.KindOrigin, value: "*"},
		{name: "unknown kind", kind: "department", value: "x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := scope.New(tc.kind, tc.value)
			require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
		})
	}

	_, err := scope.Parse("origin-without-separator")
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}
