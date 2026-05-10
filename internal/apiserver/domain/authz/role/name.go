package role

import (
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// Name is the stable business identifier of a role.
type Name string

func NewName(value string) (Name, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", perrors.WithCode(code.ErrInvalidArgument, "role name is required")
	}
	return Name(value), nil
}

func (n Name) String() string {
	return string(n)
}

// App returns the optional app namespace encoded by app-scoped role names.
func (n Name) App() (string, bool) {
	value := strings.TrimSpace(string(n))
	if value == "" {
		return "", false
	}
	app, _, ok := strings.Cut(value, ":")
	if !ok || app == "" {
		return "", false
	}
	return app, true
}

func AppName(value string) (string, bool) {
	name, err := NewName(value)
	if err != nil {
		return "", false
	}
	return name.App()
}
