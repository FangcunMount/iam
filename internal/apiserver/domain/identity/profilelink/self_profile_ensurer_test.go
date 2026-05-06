package profilelink

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
	userdomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type selfProfileCreatorStub struct {
	created []*profile.Profile
	err     error
}

func (s *selfProfileCreatorStub) Create(_ context.Context, p *profile.Profile) error {
	s.created = append(s.created, p)
	return s.err
}

type selfProfileLinkRepoStub struct {
	existing  []*ProfileLink
	created   []*ProfileLink
	updated   []*ProfileLink
	updateErr error
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
func (s *selfProfileLinkRepoStub) Update(_ context.Context, link *ProfileLink) error {
	s.updated = append(s.updated, link)
	return s.updateErr
}

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

func TestSelfProfileEnsurerReturnsProfileCreateErrorWithoutCreatingLink(t *testing.T) {
	t.Parallel()

	u, err := userdomain.NewUser("Alice", meta.Phone{}, userdomain.WithID(meta.FromUint64(10)))
	require.NoError(t, err)
	profiles := &selfProfileCreatorStub{err: errors.New("create profile failed")}
	links := &selfProfileLinkRepoStub{}

	err = NewSelfProfileEnsurer(profiles, links).Ensure(context.Background(), u)

	require.Error(t, err)
	require.Contains(t, err.Error(), "create profile failed")
	require.Len(t, profiles.created, 1)
	require.Empty(t, links.created)
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

func TestSelfProfileEnsurerConvertsDuplicateActiveSelfLinksToParentRelations(t *testing.T) {
	t.Parallel()

	u, err := userdomain.NewUser("Alice", meta.Phone{}, userdomain.WithID(meta.FromUint64(10)))
	require.NoError(t, err)
	earliest := NewSelfProfileLink(meta.FromUint64(10), meta.FromUint64(20), time.Unix(1, 0))
	earliest.ID = meta.FromUint64(1)
	duplicate := NewSelfProfileLink(meta.FromUint64(10), meta.FromUint64(21), time.Unix(2, 0))
	duplicate.ID = meta.FromUint64(2)
	links := &selfProfileLinkRepoStub{existing: []*ProfileLink{duplicate, earliest}}

	err = NewSelfProfileEnsurer(&selfProfileCreatorStub{}, links).Ensure(context.Background(), u)

	require.NoError(t, err)
	require.Empty(t, links.created)
	require.Len(t, links.updated, 1)
	require.Equal(t, TypeSelf, earliest.Type)
	require.Equal(t, RelSelf, earliest.Rel)
	require.Equal(t, duplicate, links.updated[0])
	require.Equal(t, TypeRelation, duplicate.Type)
	require.Equal(t, RelParent, duplicate.Rel)
}

func TestSelfProfileEnsurerReturnsUpdateErrorWhenConvertingDuplicateSelfLink(t *testing.T) {
	t.Parallel()

	u, err := userdomain.NewUser("Alice", meta.Phone{}, userdomain.WithID(meta.FromUint64(10)))
	require.NoError(t, err)
	links := &selfProfileLinkRepoStub{
		existing: []*ProfileLink{
			NewSelfProfileLink(meta.FromUint64(10), meta.FromUint64(20), time.Unix(1, 0)),
			NewSelfProfileLink(meta.FromUint64(10), meta.FromUint64(21), time.Unix(2, 0)),
		},
		updateErr: errors.New("duplicate relation"),
	}

	err = NewSelfProfileEnsurer(&selfProfileCreatorStub{}, links).Ensure(context.Background(), u)

	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate relation")
	require.Len(t, links.updated, 1)
}
