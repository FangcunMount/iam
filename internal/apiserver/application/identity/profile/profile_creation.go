package profile

import (
	"strings"

	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/input"
	profiledomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type profileCreationInput struct {
	Name     string
	Gender   uint8
	Birthday string
	IDCard   meta.IDCard
}

func buildProfileEntity(in profileCreationInput) (*profiledomain.Profile, error) {
	opts := []profiledomain.ProfileOption{
		profiledomain.WithGender(input.ParseGender(in.Gender)),
		profiledomain.WithBirthday(input.ParseBirthday(strings.TrimSpace(in.Birthday))),
	}
	if in.IDCard.String() != "" {
		opts = append(opts, profiledomain.WithIDCard(in.IDCard))
	}

	return profiledomain.NewProfile(strings.TrimSpace(in.Name), opts...)
}
