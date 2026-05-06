package profile

import (
	"context"
	"strings"

	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/input"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
	profiledomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
)

type profileCreationInput struct {
	Name     string
	Gender   uint8
	Birthday string
	IDCard   string
	Height   *uint32
	Weight   *uint32
}

func buildProfileEntity(txCtx context.Context, tx uow.TxRepositories, in profileCreationInput) (*profiledomain.Profile, error) {
	spec, err := buildProfileCreationSpec(in)
	if err != nil {
		return nil, err
	}
	validator := profiledomain.NewValidator(tx.Profiles)
	if err := validator.ValidateCreate(txCtx, spec.Name, spec.Gender, spec.Birthday); err != nil {
		return nil, err
	}
	return profiledomain.NewFromCreationSpec(spec)
}

func buildProfileCreationSpec(in profileCreationInput) (profiledomain.CreationSpec, error) {
	name := strings.TrimSpace(in.Name)
	spec := profiledomain.CreationSpec{
		Name:     name,
		Gender:   input.ParseGender(in.Gender),
		Birthday: input.ParseBirthday(strings.TrimSpace(in.Birthday)),
	}

	if strings.TrimSpace(in.IDCard) != "" {
		idCard, err := input.ParseIDCard(name, strings.TrimSpace(in.IDCard))
		if err != nil {
			return profiledomain.CreationSpec{}, err
		}
		spec.IDCard = idCard
	}

	if in.Height != nil {
		height, err := input.ParseHeightCm(*in.Height)
		if err != nil {
			return profiledomain.CreationSpec{}, err
		}
		spec.Height = height
	}
	if in.Weight != nil {
		weight, err := input.ParseWeightGrams(*in.Weight)
		if err != nil {
			return profiledomain.CreationSpec{}, err
		}
		spec.Weight = weight
	}

	return spec, nil
}
