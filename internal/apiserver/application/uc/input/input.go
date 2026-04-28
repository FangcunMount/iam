package input

import (
	"fmt"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ParseUserID preserves the legacy application parsing behavior based on fmt.Sscanf.
func ParseUserID(raw string) (meta.ID, error) {
	return parseID(raw)
}

// ParseChildID preserves the legacy application parsing behavior based on fmt.Sscanf.
func ParseChildID(raw string) (meta.ID, error) {
	return parseID(raw)
}

func parseID(raw string) (meta.ID, error) {
	var id uint64
	if _, err := fmt.Sscanf(raw, "%d", &id); err != nil {
		return meta.FromUint64(0), err
	}
	return meta.FromUint64(id), nil
}

func ParseOptionalPhone(raw string) (meta.Phone, error) {
	if raw == "" {
		return meta.Phone{}, nil
	}
	return meta.NewPhone(raw)
}

func ParseOptionalEmail(raw string) (meta.Email, error) {
	if raw == "" {
		return meta.Email{}, nil
	}
	return meta.NewEmail(raw)
}

func ParseOptionalIDCard(name, raw string) (meta.IDCard, bool, error) {
	if raw == "" {
		return meta.IDCard{}, false, nil
	}
	idCard, err := meta.NewIDCard(name, raw)
	return idCard, true, err
}

func ParseIDCard(name, raw string) (meta.IDCard, error) {
	return meta.NewIDCard(name, raw)
}

func ParseGender(raw uint8) meta.Gender {
	return meta.NewGender(raw)
}

func ParseBirthday(raw string) meta.Birthday {
	return meta.NewBirthday(raw)
}

func ParseHeightCm(raw uint32) (meta.Height, error) {
	return meta.NewHeightFromFloat(float64(raw))
}

func ParseOptionalHeightCm(raw *uint32) (meta.Height, bool, error) {
	if raw == nil {
		return meta.Height{}, false, nil
	}
	height, err := ParseHeightCm(*raw)
	return height, true, err
}

func ParseWeightGrams(raw uint32) (meta.Weight, error) {
	return meta.NewWeightFromFloat(float64(raw) / 1000.0)
}

func ParseOptionalWeightGrams(raw *uint32) (meta.Weight, bool, error) {
	if raw == nil {
		return meta.Weight{}, false, nil
	}
	weight, err := ParseWeightGrams(*raw)
	return weight, true, err
}

func HeightCm(height meta.Height) uint32 {
	return uint32(height.Tenths() / 10)
}

func WeightGrams(weight meta.Weight) uint32 {
	return uint32(weight.Tenths() * 100)
}
