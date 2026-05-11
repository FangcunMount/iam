package decision

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestRequestUsesTypedResourcePatternAndBusinessAction(t *testing.T) {
	t.Parallel()

	sub, err := subject.NewUserRef(meta.FromUint64(100))
	require.NoError(t, err)
	request, err := NewRequest(sub, " tenant-a ", " qs:*:*:* ", " read ")
	require.NoError(t, err)
	require.Equal(t, "tenant-a", request.TenantIDString())
	require.Equal(t, "qs:*:*:*", request.ResourceKeyString())
	require.Equal(t, "read", request.ActionString())
	require.Equal(t, scope.Default(), request.ObjectScope)
}

func TestRequestRejectsPolicyActionPatterns(t *testing.T) {
	t.Parallel()

	sub, err := subject.NewUserRef(meta.FromUint64(100))
	require.NoError(t, err)

	_, err = NewRequest(sub, "tenant-a", "iam:identity:collection:users", "read|list")
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	_, err = NewRequest(sub, "tenant-a", "iam:identity:collection:users", ".*")
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}
