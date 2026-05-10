package resource

import (
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

const keySegmentCount = 4

// Key identifies a protected resource in the catalog.
type Key string

// Pattern identifies a resource object or resource family in authorization facts.
type Pattern string

func NewKey(value string) (Key, error) {
	value = strings.TrimSpace(value)
	if err := validateFourSegmentResource(value, "resource key"); err != nil {
		return "", err
	}
	return Key(value), nil
}

func NewPattern(value string) (Pattern, error) {
	value = strings.TrimSpace(value)
	if err := validateFourSegmentResource(value, "resource pattern"); err != nil {
		return "", err
	}
	return Pattern(value), nil
}

func validateFourSegmentResource(value, label string) error {
	if value == "" {
		return perrors.WithCode(code.ErrInvalidArgument, "%s is required", label)
	}
	parts := strings.Split(value, ":")
	if len(parts) != keySegmentCount {
		return perrors.WithCode(code.ErrInvalidArgument, "%s must use <app>:<domain>:<type>:<name-or-pattern>", label)
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return perrors.WithCode(code.ErrInvalidArgument, "%s contains empty segment", label)
		}
	}
	return nil
}

func (k Key) String() string {
	return string(k)
}

func (k Key) App() string {
	return appSegment(string(k))
}

func (k Key) Domain() string {
	return segment(string(k), 1)
}

func (k Key) Type() string {
	return segment(string(k), 2)
}

func (p Pattern) String() string {
	return string(p)
}

func (p Pattern) App() string {
	return appSegment(string(p))
}

func AppNameFromKey(value string) (string, bool) {
	key, err := NewKey(value)
	if err != nil {
		pattern, patternErr := NewPattern(value)
		if patternErr != nil {
			return "", false
		}
		app := pattern.App()
		return app, app != "" && app != "*"
	}
	app := key.App()
	return app, app != "" && app != "*"
}

func appSegment(value string) string {
	return segment(value, 0)
}

func segment(value string, index int) string {
	parts := strings.Split(value, ":")
	if len(parts) != keySegmentCount {
		return ""
	}
	return parts[index]
}
