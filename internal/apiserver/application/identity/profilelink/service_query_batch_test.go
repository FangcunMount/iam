package profilelink_test

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/profilelink"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/uow"
	profiledomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/profile"
	linkdomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/profilelink"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestDirectoryListProfilesForUserUsesBatchProfileLookupAndKeepsLinkOrder(t *testing.T) {
	profile1, err := profiledomain.NewProfile("first", profiledomain.WithProfileID(meta.FromUint64(101)))
	require.NoError(t, err)
	profile2, err := profiledomain.NewProfile("second", profiledomain.WithProfileID(meta.FromUint64(102)))
	require.NoError(t, err)

	profiles := &batchProfileRepoStub{
		profiles: map[meta.ID]*profiledomain.Profile{
			profile1.ID: profile1,
			profile2.ID: profile2,
		},
	}
	links := &profileLinkRepoStub{
		byUser: []*linkdomain.ProfileLink{
			{ID: meta.FromUint64(1), User: meta.FromUint64(10), Profile: profile2.ID, Rel: linkdomain.RelParent, EstablishedAt: time.Unix(2, 0)},
			{ID: meta.FromUint64(2), User: meta.FromUint64(10), Profile: profile1.ID, Rel: linkdomain.RelParent, EstablishedAt: time.Unix(1, 0)},
		},
	}
	directory := profilelink.NewDirectory(profileLinkUOWStub{profiles: profiles, links: links})

	results, err := directory.ListProfilesForUser(context.Background(), meta.FromUint64(10))

	require.NoError(t, err)
	require.Equal(t, 1, profiles.findByIDsCalls)
	require.Zero(t, profiles.findByIDCalls)
	require.Equal(t, []meta.ID{profile2.ID, profile1.ID}, profiles.findByIDsArg)
	require.Len(t, results, 2)
	require.Equal(t, profile2.ID.String(), results[0].ProfileID)
	require.Equal(t, profile1.ID.String(), results[1].ProfileID)
}

type profileLinkUOWStub struct {
	profiles profiledomain.Repository
	links    linkdomain.Repository
}

func (s profileLinkUOWStub) WithinTx(ctx context.Context, fn func(txCtx context.Context, tx uow.TxRepositories) error) error {
	return fn(ctx, uow.TxRepositories{
		Profiles:     s.profiles,
		ProfileLinks: s.links,
	})
}

type batchProfileRepoStub struct {
	profiles       map[meta.ID]*profiledomain.Profile
	findByIDCalls  int
	findByIDsCalls int
	findByIDsArg   []meta.ID
}

func (s *batchProfileRepoStub) Create(context.Context, *profiledomain.Profile) error { return nil }
func (s *batchProfileRepoStub) FindByID(context.Context, meta.ID) (*profiledomain.Profile, error) {
	s.findByIDCalls++
	return nil, nil
}
func (s *batchProfileRepoStub) FindByIDs(_ context.Context, ids []meta.ID) (map[meta.ID]*profiledomain.Profile, error) {
	s.findByIDsCalls++
	s.findByIDsArg = append([]meta.ID(nil), ids...)
	return s.profiles, nil
}
func (s *batchProfileRepoStub) FindByName(context.Context, string) (*profiledomain.Profile, error) {
	return nil, nil
}
func (s *batchProfileRepoStub) FindByIDCard(context.Context, meta.IDCard) (*profiledomain.Profile, error) {
	return nil, nil
}
func (s *batchProfileRepoStub) FindListByName(context.Context, string) ([]*profiledomain.Profile, error) {
	return nil, nil
}
func (s *batchProfileRepoStub) FindListByNameAndBirthday(context.Context, string, meta.Birthday) ([]*profiledomain.Profile, error) {
	return nil, nil
}
func (s *batchProfileRepoStub) Update(context.Context, *profiledomain.Profile) error { return nil }

type profileLinkRepoStub struct {
	byUser []*linkdomain.ProfileLink
}

func (s *profileLinkRepoStub) Create(context.Context, *linkdomain.ProfileLink) error { return nil }
func (s *profileLinkRepoStub) FindByID(context.Context, meta.ID) (*linkdomain.ProfileLink, error) {
	return nil, nil
}
func (s *profileLinkRepoStub) FindByProfileID(context.Context, meta.ID) ([]*linkdomain.ProfileLink, error) {
	return nil, nil
}
func (s *profileLinkRepoStub) FindByProfileIDIncludingRevoked(context.Context, meta.ID) ([]*linkdomain.ProfileLink, error) {
	return nil, nil
}
func (s *profileLinkRepoStub) FindByUserID(context.Context, meta.ID) ([]*linkdomain.ProfileLink, error) {
	return s.byUser, nil
}
func (s *profileLinkRepoStub) FindByUserIDIncludingRevoked(context.Context, meta.ID) ([]*linkdomain.ProfileLink, error) {
	return s.byUser, nil
}
func (s *profileLinkRepoStub) FindActiveByUserIDAndType(_ context.Context, _ meta.ID, typ linkdomain.Type) ([]*linkdomain.ProfileLink, error) {
	return filterProfileLinksByType(s.byUser, typ, false), nil
}
func (s *profileLinkRepoStub) FindByUserIDAndTypeIncludingRevoked(_ context.Context, _ meta.ID, typ linkdomain.Type) ([]*linkdomain.ProfileLink, error) {
	return filterProfileLinksByType(s.byUser, typ, true), nil
}
func (s *profileLinkRepoStub) FindByUserIDAndProfileID(context.Context, meta.ID, meta.ID) (*linkdomain.ProfileLink, error) {
	return nil, nil
}
func (s *profileLinkRepoStub) FindByUserIDAndProfileIDIncludingRevoked(context.Context, meta.ID, meta.ID) (*linkdomain.ProfileLink, error) {
	return nil, nil
}
func (s *profileLinkRepoStub) IsLinked(context.Context, meta.ID, meta.ID) (bool, error) {
	return false, nil
}
func (s *profileLinkRepoStub) Update(context.Context, *linkdomain.ProfileLink) error { return nil }

func filterProfileLinksByType(links []*linkdomain.ProfileLink, typ linkdomain.Type, includeRevoked bool) []*linkdomain.ProfileLink {
	out := make([]*linkdomain.ProfileLink, 0, len(links))
	for _, link := range links {
		if link == nil || link.Type != typ {
			continue
		}
		if !includeRevoked && !link.IsActive() {
			continue
		}
		out = append(out, link)
	}
	return out
}
