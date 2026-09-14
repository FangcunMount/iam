package profilelink_test

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v5/internal/apiserver/application/identity/profile"
	"github.com/FangcunMount/iam/v5/internal/apiserver/application/identity/profilelink"
	"github.com/FangcunMount/iam/v5/internal/apiserver/application/identity/testutil"
	"github.com/FangcunMount/iam/v5/internal/apiserver/application/identity/user"
	"github.com/FangcunMount/iam/v5/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

func TestCommands_ReestablishRevokedLink(t *testing.T) {
	for _, relation := range []string{"self", "parent"} {
		t.Run(relation, func(t *testing.T) {
			db := testutil.SetupTestDB(t)
			uow := testutil.NewUnitOfWork(db)
			ctx := context.Background()
			u, err := user.NewCreator(uow).Create(ctx, user.CreateUserDTO{Name: "restore-test"})
			require.NoError(t, err)
			p, err := testutil.NewProfileFixture(t, uow).Create(ctx, profile.CreateProfileDTO{Name: "restore-test", Gender: 1, Birthday: "2000-01-01"})
			require.NoError(t, err)
			command := profilelink.NewCommands(uow)
			query := profilelink.NewDirectory(uow)
			dto := profilelink.CreateProfileLinkDTO{UserID: mustID(t, u.ID), ProfileID: mustID(t, p.ID), Relation: relation}
			initial, err := command.Establish(ctx, dto)
			require.NoError(t, err)
			_, err = command.Revoke(ctx, profilelink.RemoveProfileLinkDTO{UserID: dto.UserID, ProfileID: dto.ProfileID})
			require.NoError(t, err)
			linked, err := query.IsLinked(ctx, dto.UserID, dto.ProfileID)
			require.NoError(t, err)
			require.False(t, linked)
			restored, err := command.Establish(ctx, dto)
			require.NoError(t, err)
			require.Equal(t, initial.ID, restored.ID)
			linked, err = query.IsLinked(ctx, dto.UserID, dto.ProfileID)
			require.NoError(t, err)
			require.True(t, linked)
			_, err = command.Establish(ctx, dto)
			require.True(t, perrors.IsCode(err, code.ErrIdentityProfileLinkExists))
			if relation == "self" {
				_, err = command.Revoke(ctx, profilelink.RemoveProfileLinkDTO{UserID: dto.UserID, ProfileID: dto.ProfileID})
				require.NoError(t, err)
				other, err := testutil.NewProfileFixture(t, uow).Create(ctx, profile.CreateProfileDTO{Name: "other-profile", Gender: 1, Birthday: "2001-01-01"})
				require.NoError(t, err)
				_, err = command.Establish(ctx, profilelink.CreateProfileLinkDTO{UserID: dto.UserID, ProfileID: mustID(t, other.ID), Relation: "self"})
				require.NoError(t, err)
				_, err = command.Establish(ctx, dto)
				require.True(t, perrors.IsCode(err, code.ErrIdentityProfileLinkExists))
				linked, err = query.IsLinked(ctx, dto.UserID, dto.ProfileID)
				require.NoError(t, err)
				require.False(t, linked)
			}
		})
	}
}
