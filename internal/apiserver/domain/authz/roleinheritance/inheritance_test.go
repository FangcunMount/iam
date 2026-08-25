package roleinheritance_test

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestInheritanceRejectsSelfEdge(t *testing.T) {
	_, err := roleinheritance.New(meta.FromUint64(1), meta.FromUint64(1), "tenant-a", "operator-1")
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestWouldCreateCycleTraversesExistingRoleGraph(t *testing.T) {
	edgeAB, err := roleinheritance.New(meta.FromUint64(1), meta.FromUint64(2), "tenant-a", "operator-1")
	require.NoError(t, err)
	edgeBC, err := roleinheritance.New(meta.FromUint64(2), meta.FromUint64(3), "tenant-a", "operator-1")
	require.NoError(t, err)

	existing := []*roleinheritance.Inheritance{&edgeAB, &edgeBC}
	require.True(t, roleinheritance.WouldCreateCycle(existing, meta.FromUint64(3), meta.FromUint64(1)))
	require.False(t, roleinheritance.WouldCreateCycle(existing, meta.FromUint64(4), meta.FromUint64(1)))
}
