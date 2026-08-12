package profilelink

import (
	"context"
	"errors"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

func TestSelfProfileGuard_EnsureCanCreateSelfAllowsWhenNoActiveSelfExists(t *testing.T) {
	relation := &ProfileLink{User: meta.FromUint64(10), Profile: meta.FromUint64(30), Type: TypeRelation, Rel: RelParent}
	guard := NewSelfProfileGuard(&stubProfileLinkRepo{
		userResults: map[uint64][]*ProfileLink{
			10: {relation},
		},
	})

	err := guard.EnsureCanCreateSelf(context.Background(), meta.FromUint64(10))

	require.NoError(t, err)
}

func TestSelfProfileGuard_EnsureCanCreateSelfRejectsExistingActiveSelf(t *testing.T) {
	existingSelf := selfProfileLink(meta.FromUint64(10), meta.FromUint64(20), time.Unix(1, 0))
	guard := NewSelfProfileGuard(&stubProfileLinkRepo{
		userResults: map[uint64][]*ProfileLink{
			10: {existingSelf},
		},
	})

	err := guard.EnsureCanCreateSelf(context.Background(), meta.FromUint64(10))

	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrIdentityProfileLinkExists))
}

func TestSelfProfileGuard_EnsureCanCreateSelfIgnoresRevokedSelf(t *testing.T) {
	revokedSelf := selfProfileLink(meta.FromUint64(10), meta.FromUint64(20), time.Unix(1, 0))
	revokedSelf.Revoke(time.Unix(2, 0))
	guard := NewSelfProfileGuard(&stubProfileLinkRepo{
		userResults: map[uint64][]*ProfileLink{
			10: {revokedSelf},
		},
	})

	err := guard.EnsureCanCreateSelf(context.Background(), meta.FromUint64(10))

	require.NoError(t, err)
}

func TestSelfProfileGuard_HasActiveSelfProfile(t *testing.T) {
	existingSelf := selfProfileLink(meta.FromUint64(10), meta.FromUint64(20), time.Unix(1, 0))
	guard := NewSelfProfileGuard(&stubProfileLinkRepo{
		userResults: map[uint64][]*ProfileLink{
			10: {existingSelf},
		},
	})

	hasSelf, err := guard.HasActiveSelfProfile(context.Background(), meta.FromUint64(10))

	require.NoError(t, err)
	assert.True(t, hasSelf)
}

func selfProfileLink(userID meta.ID, profileID meta.ID, establishedAt time.Time) *ProfileLink {
	return &ProfileLink{
		User:          userID,
		Profile:       profileID,
		Type:          TypeSelf,
		Rel:           RelSelf,
		EstablishedAt: establishedAt,
	}
}

func TestSelfProfileGuard_PropagatesRepositoryErrors(t *testing.T) {
	guard := NewSelfProfileGuard(&stubProfileLinkRepo{findByUserErr: errors.New("link db down")})

	err := guard.EnsureCanCreateSelf(context.Background(), meta.FromUint64(10))

	require.Error(t, err)
	assert.True(t, perrors.IsCode(err, code.ErrDatabase))
}
