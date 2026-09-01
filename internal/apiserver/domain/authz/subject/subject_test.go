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

func TestParseRefAcceptsCanonicalSubject(t *testing.T) {
	ref, err := subject.ParseRef(" user:42 ")
	require.NoError(t, err)
	require.Equal(t, subject.TypeUser, ref.Type)
	require.Equal(t, meta.FromUint64(42), ref.ID)
}

func TestParseRefRejectsMalformedSubject(t *testing.T) {
	for _, value := range []string{"", "user", "user:", "robot:42", "user:not-an-id"} {
		_, err := subject.ParseRef(value)
		require.True(t, perrors.IsCode(err, code.ErrInvalidArgument), value)
	}
}
