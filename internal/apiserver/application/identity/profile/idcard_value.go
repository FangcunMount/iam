package profile

import "github.com/FangcunMount/iam/v2/internal/pkg/meta"

func optionalIDCard(name, raw string) (meta.IDCard, bool, error) {
	if raw == "" {
		return meta.IDCard{}, false, nil
	}
	idCard, err := meta.NewIDCard(name, raw)
	return idCard, true, err
}
