package credential

import (
	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

type binder struct{}

var _ Binder = (*binder)(nil)

func NewBinder() Binder {
	return &binder{}
}

func (b *binder) Bind(spec BindSpec) (*Credential, error) {
	if spec.LoginIdentityID.IsZero() {
		return nil, errors.WithCode(code.ErrInvalidArgument, "login identity ID cannot be zero")
	}
	if spec.Type == "" {
		return nil, errors.WithCode(code.ErrInvalidArgument, "credential type cannot be empty")
	}
	if spec.Type != CredPassword {
		return nil, errors.WithCode(code.ErrInvalidCredential, "unsupported persistent credential type: %s", spec.Type)
	}
	if len(spec.Material) == 0 {
		return nil, errors.WithCode(code.ErrInvalidCredential, "password credential requires material")
	}
	if spec.Algo == nil || *spec.Algo == "" {
		return nil, errors.WithCode(code.ErrInvalidCredential, "password credential requires algo")
	}

	return &Credential{
		LoginIdentityID: spec.LoginIdentityID,
		Type:            spec.Type,
		Material:        spec.Material,
		Algo:            spec.Algo,
		ParamsJSON:      spec.ParamsJSON,
		Status:          CredStatusEnabled,
	}, nil
}
