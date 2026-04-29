package identity

import (
	"strconv"

	identityv1 "github.com/FangcunMount/iam/api/grpc/iam/identity/v1"
	childApp "github.com/FangcunMount/iam/internal/apiserver/application/uc/child"
	guardianshipApp "github.com/FangcunMount/iam/internal/apiserver/application/uc/guardianship"
)

func childResultToProto(result *childApp.ChildResult) *identityv1.Child {
	if result == nil {
		return nil
	}

	return &identityv1.Child{
		Id:        result.ID,
		LegalName: result.Name,
		Gender:    genderUint8ToProto(result.Gender),
		Dob:       result.Birthday,
		Identity: &identityv1.IdentityDocument{
			Type:         "id_card",
			MaskedNumber: result.IDCard,
		},
		Stats: &identityv1.PhysicalStats{
			HeightCm: int32(result.Height),
			WeightKg: formatWeight(result.Weight),
		},
		CreatedAt: nil,
		UpdatedAt: nil,
	}
}

func childResultToProtoFromGuardianship(result *guardianshipApp.GuardianshipResult) *identityv1.Child {
	if result == nil {
		return nil
	}

	return &identityv1.Child{
		Id:        result.ChildID,
		LegalName: result.ChildName,
		Gender:    genderUint8ToProto(result.ChildGender),
		Dob:       result.ChildBirthday,
		Identity:  nil,
		Stats:     nil,
		CreatedAt: nil,
		UpdatedAt: nil,
	}
}

func genderUint8ToProto(gender uint8) identityv1.Gender {
	switch gender {
	case 1:
		return identityv1.Gender_GENDER_MALE
	case 2:
		return identityv1.Gender_GENDER_FEMALE
	case 0:
		return identityv1.Gender_GENDER_OTHER
	default:
		return identityv1.Gender_GENDER_UNSPECIFIED
	}
}

func formatWeight(weightGrams uint32) string {
	if weightGrams == 0 {
		return ""
	}
	kg := float64(weightGrams) / 1000.0
	return strconv.FormatFloat(kg, 'f', 2, 64)
}
