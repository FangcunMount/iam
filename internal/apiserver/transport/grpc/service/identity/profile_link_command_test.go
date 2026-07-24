package identity

import (
	"context"
	"errors"
	"strings"
	"testing"

	identityv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/identity/v2"
	profileLinkApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profilelink"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type profileLinkCommandStub struct {
	establishCalls       []profileLinkApp.CreateProfileLinkDTO
	revokeBySelectorCall []profileLinkApp.RevokeProfileLinkBySelectorDTO
	establishErr         error
	revokeErr            error
}

func (s *profileLinkCommandStub) Establish(_ context.Context, dto profileLinkApp.CreateProfileLinkDTO) (*profileLinkApp.ProfileLinkResult, error) {
	s.establishCalls = append(s.establishCalls, dto)
	if s.establishErr != nil {
		return nil, s.establishErr
	}
	return &profileLinkApp.ProfileLinkResult{
		ID:            1,
		UserID:        dto.UserID.String(),
		ProfileID:     dto.ProfileID.String(),
		Relation:      dto.Relation,
		EstablishedAt: "2026-04-29T10:20:30Z",
	}, nil
}

func (s *profileLinkCommandStub) Revoke(_ context.Context, dto profileLinkApp.RemoveProfileLinkDTO) (*profileLinkApp.ProfileLinkResult, error) {
	return &profileLinkApp.ProfileLinkResult{
		ID:            1,
		UserID:        dto.UserID.String(),
		ProfileID:     dto.ProfileID.String(),
		Relation:      "parent",
		EstablishedAt: "2026-04-29T10:20:30Z",
		RevokedAt:     "2026-04-29T11:20:30Z",
	}, nil
}

func (s *profileLinkCommandStub) RevokeBySelector(_ context.Context, dto profileLinkApp.RevokeProfileLinkBySelectorDTO) (*profileLinkApp.ProfileLinkResult, error) {
	s.revokeBySelectorCall = append(s.revokeBySelectorCall, dto)
	if s.revokeErr != nil {
		return nil, s.revokeErr
	}
	return &profileLinkApp.ProfileLinkResult{
		ID:            2,
		UserID:        dto.UserID.String(),
		ProfileID:     dto.ProfileID.String(),
		Relation:      "parent",
		EstablishedAt: "2026-04-29T10:20:30Z",
		RevokedAt:     "2026-04-29T11:20:30Z",
	}, nil
}

func TestProfileLinkBatchFailuresHideInternalErrorText(t *testing.T) {
	const sentinel = "profile-link-internal-error-sentinel"
	commands := &profileLinkCommandStub{
		establishErr: errors.New(sentinel),
		revokeErr:    errors.New(sentinel),
	}
	server := &profileLinkCommandServer{profileLinkSvc: commands}

	imported, err := server.ImportProfileLinks(context.Background(), &identityv2.ImportProfileLinksRequest{
		Records: []*identityv2.ImportProfileLinkRecord{{
			UserId:    "100",
			ProfileId: "200",
		}},
	})
	require.NoError(t, err)
	require.Len(t, imported.GetFailures(), 1)
	require.Equal(t, "internal server error", imported.GetFailures()[0].GetError())

	revoked, err := server.BatchRevokeProfileLinks(context.Background(), &identityv2.BatchRevokeProfileLinksRequest{
		Targets: []*identityv2.ProfileLinkSelector{{
			Selector: &identityv2.ProfileLinkSelector_Key{
				Key: &identityv2.ProfileLinkKey{UserId: "100", ProfileId: "200"},
			},
		}},
	})
	require.NoError(t, err)
	require.Len(t, revoked.GetFailures(), 1)
	require.Equal(t, "internal server error", revoked.GetFailures()[0].GetError())
	require.False(t, strings.Contains(imported.GetFailures()[0].GetError(), sentinel))
	require.False(t, strings.Contains(revoked.GetFailures()[0].GetError(), sentinel))
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
	require.Equal(t, meta.FromUint64(100), commands.establishCalls[0].UserID)
	require.Equal(t, meta.FromUint64(200), commands.establishCalls[0].ProfileID)
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
	require.Equal(t, meta.FromUint64(100), commands.revokeBySelectorCall[0].UserID)
	require.Equal(t, meta.FromUint64(200), commands.revokeBySelectorCall[0].ProfileID)
}
