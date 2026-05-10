package tenant

import (
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// ID identifies the tenant/domain boundary for authorization.
type ID string

func NewID(value string) (ID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", perrors.WithCode(code.ErrInvalidArgument, "tenant id is required")
	}
	return ID(value), nil
}

func (id ID) String() string {
	return string(id)
}

func (id ID) IsZero() bool {
	return strings.TrimSpace(string(id)) == ""
}
