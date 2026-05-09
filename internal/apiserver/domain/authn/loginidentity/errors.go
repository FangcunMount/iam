package loginidentity

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

func NotFoundError() error {
	return perrors.WithCode(code.ErrNoBinding, "login identity binding not found")
}
