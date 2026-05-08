package identity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	identityv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/identity/v2"
	profileApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profile"
	profileLinkApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profilelink"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

func TestProfileCommandCreateProfileUsesMyProfilesCreate(t *testing.T) {
	stub := &profileCommandStub{}
	server := profileCommandServer{profileCommandSvc: stub}

	resp, err := server.CreateProfile(context.Background(), &identityv2.CreateProfileRequest{
		UserId:       "100",
		LegalName:    "测试档案",
		Gender:       identityv2.Gender_GENDER_MALE,
		Dob:          "2020-01-01",
		IdCardNumber: "110101202001011234",
		Relation:     identityv2.ProfileLinkRelation_PROFILE_LINK_RELATION_PARENT,
	})

	require.NoError(t, err)
	require.NotNil(t, resp.GetProfile())
	require.NotNil(t, resp.GetProfileLink())
	require.Len(t, stub.createCalls, 1)
	require.Equal(t, meta.FromUint64(100), stub.createCalls[0].userID)
	require.Equal(t, profileApp.CreateProfileDTO{
		Name:     "测试档案",
		Gender:   1,
		Birthday: "2020-01-01",
		IDCard:   "110101202001011234",
		Relation: "parent",
	}, stub.createCalls[0].dto)
	require.Equal(t, identityv2.ProfileLinkRelation_PROFILE_LINK_RELATION_PARENT, resp.GetProfileLink().GetRelation())
}

type profileCommandStub struct {
	createCalls []struct {
		userID meta.ID
		dto    profileApp.CreateProfileDTO
	}
}

func (s *profileCommandStub) Create(_ context.Context, userID meta.ID, dto profileApp.CreateProfileDTO) (*profileApp.CreatedProfileResult, error) {
	s.createCalls = append(s.createCalls, struct {
		userID meta.ID
		dto    profileApp.CreateProfileDTO
	}{userID: userID, dto: dto})
	return &profileApp.CreatedProfileResult{
		Profile: &profileApp.ProfileResult{
			ID:       "200",
			Name:     dto.Name,
			Gender:   dto.Gender,
			Birthday: dto.Birthday,
			IDCard:   dto.IDCard,
		},
		ProfileLink: &profileLinkApp.ProfileLinkResult{
			ID:        300,
			UserID:    userID.String(),
			ProfileID: "200",
			Relation:  dto.Relation,
		},
	}, nil
}

func (s *profileCommandStub) List(context.Context, meta.ID) ([]*profileApp.ProfileResult, error) {
	return nil, nil
}

func (s *profileCommandStub) Get(context.Context, meta.ID, meta.ID) (*profileApp.ProfileResult, error) {
	return nil, nil
}

func (s *profileCommandStub) Patch(context.Context, profileApp.PatchMyProfileDTO) (*profileApp.ProfileResult, error) {
	return nil, nil
}
