package testutil

import (
	"context"
	"testing"

	profileapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/profile"
	identityuow "github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/uow"
	profiledomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/profile"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// ProfileFixture creates Profile records directly for application tests that
// need an existing profile without exercising the public Profile+ProfileLink use case.
type ProfileFixture struct {
	uow identityuow.UnitOfWork
}

func NewProfileFixture(t *testing.T, uow identityuow.UnitOfWork) *ProfileFixture {
	t.Helper()
	return &ProfileFixture{uow: uow}
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
