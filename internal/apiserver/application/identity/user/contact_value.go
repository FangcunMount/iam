package user

import "github.com/FangcunMount/iam/v2/internal/pkg/meta"

func optionalPhone(raw string) (meta.Phone, error) {
	if raw == "" {
		return meta.Phone{}, nil
	}
	return meta.NewPhone(raw)
}

func optionalEmail(raw string) (meta.Email, error) {
	if raw == "" {
		return meta.Email{}, nil
	}
	return meta.NewEmail(raw)
}
