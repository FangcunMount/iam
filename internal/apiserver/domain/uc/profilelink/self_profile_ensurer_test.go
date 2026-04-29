package profilelink

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/internal/apiserver/domain/uc/profile"
	userdomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type selfProfileCreatorStub struct {
	created []*profile.Profile
}

func (s *selfProfileCreatorStub) Create(_ context.Context, p *profile.Profile) error {
	s.created = append(s.created, p)
	return nil
}

type selfProfileLinkRepoStub struct {
	existing []*ProfileLink
	created  []*ProfileLink
}

func (s *selfProfileLinkRepoStub) Create(_ context.Context, link *ProfileLink) error {
	s.created = append(s.created, link)
	return nil
}
func (s *selfProfileLinkRepoStub) FindByID(context.Context, meta.ID) (*ProfileLink, error) {
	return nil, nil
}
func (s *selfProfileLinkRepoStub) FindByProfileID(context.Context, meta.ID) ([]*ProfileLink, error) {
	return nil, nil
}
func (s *selfProfileLinkRepoStub) FindByProfileIDIncludingRevoked(context.Context, meta.ID) ([]*ProfileLink, error) {
	return nil, nil
}
func (s *selfProfileLinkRepoStub) FindByUserID(context.Context, meta.ID) ([]*ProfileLink, error) {
	return s.existing, nil
}
func (s *selfProfileLinkRepoStub) FindByUserIDIncludingRevoked(context.Context, meta.ID) ([]*ProfileLink, error) {
	return s.existing, nil
}
func (s *selfProfileLinkRepoStub) FindByUserIDAndProfileID(context.Context, meta.ID, meta.ID) (*ProfileLink, error) {
	return nil, nil
}
func (s *selfProfileLinkRepoStub) FindByUserIDAndProfileIDIncludingRevoked(context.Context, meta.ID, meta.ID) (*ProfileLink, error) {
	return nil, nil
}
func (s *selfProfileLinkRepoStub) IsLinked(context.Context, meta.ID, meta.ID) (bool, error) {
	return false, nil
}
func (s *selfProfileLinkRepoStub) Update(context.Context, *ProfileLink) error { return nil }

func TestSelfProfileEnsurerCreatesMissingSelfProfileLink(t *testing.T) {
	t.Parallel()

	u, err := userdomain.NewUser("Alice", meta.Phone{}, userdomain.WithID(meta.FromUint64(10)))
	require.NoError(t, err)
	profiles := &selfProfileCreatorStub{}
	links := &selfProfileLinkRepoStub{}
	ensurer := NewSelfProfileEnsurer(profiles, links)
	ensurer.now = func() time.Time { return time.Unix(100, 0) }

	err = ensurer.Ensure(context.Background(), u)
	require.NoError(t, err)
	require.Len(t, profiles.created, 1)
	require.Equal(t, "Alice", profiles.created[0].Name)
	require.Len(t, links.created, 1)
	require.Equal(t, TypeSelf, links.created[0].Type)
	require.Equal(t, RelSelf, links.created[0].Rel)
	require.Equal(t, meta.FromUint64(10), links.created[0].User)
	require.Equal(t, profiles.created[0].ID, links.created[0].Profile)
	require.Equal(t, time.Unix(100, 0), links.created[0].EstablishedAt)
}

func TestSelfProfileEnsurerSkipsExistingActiveSelfLink(t *testing.T) {
	t.Parallel()

	u, err := userdomain.NewUser("Alice", meta.Phone{}, userdomain.WithID(meta.FromUint64(10)))
	require.NoError(t, err)
	profiles := &selfProfileCreatorStub{}
	links := &selfProfileLinkRepoStub{
		existing: []*ProfileLink{NewSelfProfileLink(meta.FromUint64(10), meta.FromUint64(20), time.Unix(1, 0))},
	}

	err = NewSelfProfileEnsurer(profiles, links).Ensure(context.Background(), u)
	require.NoError(t, err)
	require.Empty(t, profiles.created)
	require.Empty(t, links.created)
}
