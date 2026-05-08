package testutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	profileapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profile"
	identityuow "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
	profiledomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ProfileFixture creates Profile records directly for application tests that
// need an existing profile without exercising the public Profile+ProfileLink use case.
type ProfileFixture struct {
	t   *testing.T
	uow identityuow.UnitOfWork
}

func NewProfileFixture(t *testing.T, uow identityuow.UnitOfWork) *ProfileFixture {
	t.Helper()
	return &ProfileFixture{t: t, uow: uow}
}

func (f *ProfileFixture) Create(ctx context.Context, dto profileapp.CreateProfileDTO) (*profileapp.ProfileResult, error) {
	var result *profileapp.ProfileResult

	err := f.uow.WithinTx(ctx, func(txCtx context.Context, tx identityuow.TxRepositories) error {
		newProfile, err := newProfileFromDTO(dto)
		if err != nil {
			return err
		}
		if err := tx.Profiles.Create(txCtx, newProfile); err != nil {
			return err
		}

		result = &profileapp.ProfileResult{
			ID:       newProfile.ID.String(),
			Name:     newProfile.Name,
			IDCard:   newProfile.IDCard.Number(),
			Gender:   uint8(newProfile.Gender),
			Birthday: newProfile.Birthday.String(),
		}
		return nil
	})

	return result, err
}

func (f *ProfileFixture) MustCreate(ctx context.Context, dto profileapp.CreateProfileDTO) *profileapp.ProfileResult {
	f.t.Helper()
	result, err := f.Create(ctx, dto)
	require.NoError(f.t, err)
	return result
}

func newProfileFromDTO(dto profileapp.CreateProfileDTO) (*profiledomain.Profile, error) {
	opts := make([]profiledomain.ProfileOption, 0, 3)
	if dto.Gender != 0 {
		opts = append(opts, profiledomain.WithGender(meta.NewGender(dto.Gender)))
	}
	if dto.Birthday != "" {
		opts = append(opts, profiledomain.WithBirthday(meta.NewBirthday(dto.Birthday)))
	}
	if dto.IDCard != "" {
		idCard, err := meta.NewIDCard(dto.Name, dto.IDCard)
		if err != nil {
			return nil, err
		}
		opts = append(opts, profiledomain.WithIDCard(idCard))
	}

	return profiledomain.NewProfile(dto.Name, opts...)
}
