package profile

import (
	"strings"

	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type profileCreationInput struct {
	Name     string
	Gender   meta.Gender
	Birthday meta.Birthday
	IDCard   meta.IDCard
}

func buildProfileEntity(in profileCreationInput) (*domain.Profile, error) {
	opts := make([]domain.ProfileOption, 0, 3)
	if in.Gender.IsValid() {
		opts = append(opts, domain.WithGender(in.Gender))
	}
	if in.Birthday.IsValid() {
		opts = append(opts, domain.WithBirthday(in.Birthday))
	}
	if in.IDCard.IsValid() {
		opts = append(opts, domain.WithIDCard(in.IDCard))
	}

	return domain.NewProfile(strings.TrimSpace(in.Name), opts...)
}
