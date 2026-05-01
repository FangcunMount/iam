package identity

import (
	"context"
	"testing"

	identityv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/identity/v2"
	profileLinkApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/uc/profilelink"
	"github.com/stretchr/testify/require"
)

type profileLinkCommandStub struct {
	establishCalls       []profileLinkApp.CreateProfileLinkDTO
	revokeBySelectorCall []profileLinkApp.RevokeProfileLinkBySelectorDTO
}

func (s *profileLinkCommandStub) Establish(_ context.Context, dto profileLinkApp.CreateProfileLinkDTO) (*profileLinkApp.ProfileLinkResult, error) {
	s.establishCalls = append(s.establishCalls, dto)
	return &profileLinkApp.ProfileLinkResult{
		ID:            1,
		UserID:        dto.UserID,
		ProfileID:     dto.ProfileID,
		Relation:      dto.Relation,
		EstablishedAt: "2026-04-29T10:20:30Z",
	}, nil
}

func (s *profileLinkCommandStub) Revoke(_ context.Context, dto profileLinkApp.RemoveProfileLinkDTO) (*profileLinkApp.ProfileLinkResult, error) {
	return &profileLinkApp.ProfileLinkResult{
		ID:            1,
		UserID:        dto.UserID,
		ProfileID:     dto.ProfileID,
		Relation:      "parent",
		EstablishedAt: "2026-04-29T10:20:30Z",
		RevokedAt:     "2026-04-29T11:20:30Z",
	}, nil
}

func (s *profileLinkCommandStub) RevokeBySelector(_ context.Context, dto profileLinkApp.RevokeProfileLinkBySelectorDTO) (*profileLinkApp.ProfileLinkResult, error) {
	s.revokeBySelectorCall = append(s.revokeBySelectorCall, dto)
	return &profileLinkApp.ProfileLinkResult{
		ID:            2,
		UserID:        dto.UserID,
		ProfileID:     dto.ProfileID,
		Relation:      "parent",
		EstablishedAt: "2026-04-29T10:20:30Z",
		RevokedAt:     "2026-04-29T11:20:30Z",
	}, nil
}

func TestProfileLinkCommandEstablishUsesSystemCommand(t *testing.T) {
	commands := &profileLinkCommandStub{}
	server := &profileLinkCommandServer{profileLinkSvc: commands}

	resp, err := server.EstablishProfileLink(context.Background(), &identityv2.EstablishProfileLinkRequest{
		UserId:    "100",
		ProfileId: "200",
		Relation:  identityv2.ProfileLinkRelation_PROFILE_LINK_RELATION_PARENT,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, commands.establishCalls, 1)
	require.Equal(t, "100", commands.establishCalls[0].UserID)
	require.Equal(t, "200", commands.establishCalls[0].ProfileID)
}

func TestProfileLinkCommandRevokeUsesSystemCommandSelector(t *testing.T) {
	commands := &profileLinkCommandStub{}
	server := &profileLinkCommandServer{profileLinkSvc: commands}

	resp, err := server.RevokeProfileLink(context.Background(), &identityv2.RevokeProfileLinkRequest{
		Target: &identityv2.ProfileLinkSelector{
			Selector: &identityv2.ProfileLinkSelector_Key{
				Key: &identityv2.ProfileLinkKey{
					UserId:    "100",
					ProfileId: "200",
				},
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, commands.revokeBySelectorCall, 1)
	require.Equal(t, "100", commands.revokeBySelectorCall[0].UserID)
	require.Equal(t, "200", commands.revokeBySelectorCall[0].ProfileID)
}
