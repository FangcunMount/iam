package subject_test

import (
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestRefNormalizesAndEncodesSupportedSubject(t *testing.T) {
	ref, err := subject.NewRef(" user ", meta.FromUint64(42))
	require.NoError(t, err)
	require.Equal(t, subject.TypeUser, ref.Type)
	require.Equal(t, "user:42", ref.String())
	require.False(t, ref.IsZero())
}

func TestRefRejectsUnsupportedOrIncompleteSubject(t *testing.T) {
	for _, tc := range []struct {
		name string
		typ  subject.Type
		id   meta.ID
	}{
		{name: "missing type", id: meta.FromUint64(1)},
		{name: "unsupported type", typ: "robot", id: meta.FromUint64(1)},
		{name: "missing id", typ: subject.TypeUser},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := subject.NewRef(tc.typ, tc.id)
			require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
		})
	}
}
