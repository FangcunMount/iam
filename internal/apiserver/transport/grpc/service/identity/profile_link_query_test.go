package identity

import (
	"context"
	"testing"

	identityv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/identity/v2"
	profileLinkApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/profilelink"
	userApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/user"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestProfileLinkQueryListProfilesHonorsIncludeRevoked(t *testing.T) {
	stub := &profileLinkQueryStub{
		active: []*profileLinkApp.ProfileLinkResult{{UserID: "10", ProfileID: "101"}},
		all:    []*profileLinkApp.ProfileLinkResult{{UserID: "10", ProfileID: "101"}, {UserID: "10", ProfileID: "102", RevokedAt: "2026-01-01T00:00:00Z"}},
	}
	server := &profileLinkQueryServer{profileLinkQuerySvc: stub}

	active, err := server.ListProfiles(context.Background(), &identityv2.ListProfilesRequest{UserId: "10"})
	require.NoError(t, err)
	require.Equal(t, int32(1), active.GetTotal())
	require.Equal(t, 1, stub.activeListCalls)

	withRevoked, err := server.ListProfiles(context.Background(), &identityv2.ListProfilesRequest{UserId: "10", IncludeRevoked: true})
	require.NoError(t, err)
	require.Equal(t, int32(2), withRevoked.GetTotal())
	require.Equal(t, 1, stub.revokedListCalls)
}

func TestProfileLinkQueryListProfileLinksUsesBatchUserLookupAndKeepsOrder(t *testing.T) {
	stub := &profileLinkQueryStub{
		active: []*profileLinkApp.ProfileLinkResult{
			{UserID: "10", ProfileID: "101", Relation: "self"},
			{UserID: "404", ProfileID: "101", Relation: "parent"},
			{UserID: "11", ProfileID: "101", Relation: "other"},
			{UserID: "10", ProfileID: "101", Relation: "self"},
		},
	}
	users := &userQueryStub{
		users: map[string]*userApp.UserResult{
			"10": {ID: "10", Name: "alice"},
			"11": {ID: "11", Name: "bob"},
		},
	}
	server := &profileLinkQueryServer{profileLinkQuerySvc: stub, userQuerySvc: users}

	resp, err := server.ListProfileLinks(context.Background(), &identityv2.ListProfileLinksRequest{ProfileId: "101"})

	require.NoError(t, err)
	require.Equal(t, int32(4), resp.GetTotal())
	require.Equal(t, []meta.ID{meta.FromUint64(10), meta.FromUint64(404), meta.FromUint64(11)}, users.batchCalls[0])
	require.Equal(t, "10", resp.GetItems()[0].GetUser().GetId())
	require.Nil(t, resp.GetItems()[1].GetUser())
	require.Equal(t, "11", resp.GetItems()[2].GetUser().GetId())
	require.Equal(t, "10", resp.GetItems()[3].GetUser().GetId())
	require.Equal(t, 0, users.getByIDCalls)
}

func TestProfileLinkQueryListProfileLinksHonorsIncludeRevoked(t *testing.T) {
	stub := &profileLinkQueryStub{
		active: []*profileLinkApp.ProfileLinkResult{{UserID: "10", ProfileID: "101"}},
		all:    []*profileLinkApp.ProfileLinkResult{{UserID: "10", ProfileID: "101"}, {UserID: "11", ProfileID: "101", RevokedAt: "2026-01-01T00:00:00Z"}},
	}
	users := &userQueryStub{users: map[string]*userApp.UserResult{}}
	server := &profileLinkQueryServer{profileLinkQuerySvc: stub, userQuerySvc: users}

	active, err := server.ListProfileLinks(context.Background(), &identityv2.ListProfileLinksRequest{ProfileId: "101"})
	require.NoError(t, err)
	require.Equal(t, int32(1), active.GetTotal())
	require.Equal(t, 1, stub.activeLinkListCalls)

	withRevoked, err := server.ListProfileLinks(context.Background(), &identityv2.ListProfileLinksRequest{ProfileId: "101", IncludeRevoked: true})
	require.NoError(t, err)
	require.Equal(t, int32(2), withRevoked.GetTotal())
	require.Equal(t, 1, stub.revokedLinkListCalls)
}

type profileLinkQueryStub struct {
	active               []*profileLinkApp.ProfileLinkResult
	all                  []*profileLinkApp.ProfileLinkResult
	activeListCalls      int
	revokedListCalls     int
	activeLinkListCalls  int
	revokedLinkListCalls int
}

func (s *profileLinkQueryStub) IsLinked(context.Context, meta.ID, meta.ID) (bool, error) {
	return false, nil
}
func (s *profileLinkQueryStub) Get(context.Context, meta.ID, meta.ID) (*profileLinkApp.ProfileLinkResult, error) {
	return nil, nil
}
func (s *profileLinkQueryStub) GetIncludingRevoked(context.Context, meta.ID, meta.ID) (*profileLinkApp.ProfileLinkResult, error) {
	return nil, nil
}
func (s *profileLinkQueryStub) ListProfilesForUser(context.Context, meta.ID) ([]*profileLinkApp.ProfileLinkResult, error) {
	s.activeListCalls++
	return s.active, nil
}
func (s *profileLinkQueryStub) ListProfilesForUserIncludingRevoked(context.Context, meta.ID) ([]*profileLinkApp.ProfileLinkResult, error) {
	s.revokedListCalls++
	return s.all, nil
}
func (s *profileLinkQueryStub) ListLinksForProfile(context.Context, meta.ID) ([]*profileLinkApp.ProfileLinkResult, error) {
	s.activeLinkListCalls++
	return s.active, nil
}
func (s *profileLinkQueryStub) ListLinksForProfileIncludingRevoked(context.Context, meta.ID) ([]*profileLinkApp.ProfileLinkResult, error) {
	s.revokedLinkListCalls++
	return s.all, nil
}

type userQueryStub struct {
	users         map[string]*userApp.UserResult
	nicknameUsers []*userApp.UserResult
	batchCalls    [][]meta.ID
	getByIDCalls  int
}

func (s *userQueryStub) GetByID(context.Context, meta.ID) (*userApp.UserResult, error) {
	s.getByIDCalls++
	return nil, nil
}
func (s *userQueryStub) BatchGetByID(_ context.Context, userIDs []meta.ID) (map[string]*userApp.UserResult, error) {
	s.batchCalls = append(s.batchCalls, append([]meta.ID(nil), userIDs...))
	return s.users, nil
}
func (s *userQueryStub) GetByPhone(context.Context, string) (*userApp.UserResult, error) {
	return nil, nil
}
func (s *userQueryStub) FindByNickname(context.Context, string) ([]*userApp.UserResult, error) {
	return s.nicknameUsers, nil
}
