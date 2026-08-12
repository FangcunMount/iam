package resource

import (
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

const keySegmentCount = 4

// Key identifies a protected resource in the catalog.
type Key string

// Pattern identifies a resource object or resource family in authorization facts.
type Pattern string

func NewKey(value string) (Key, error) {
	value = strings.TrimSpace(value)
	parts, err := parseFourSegmentResource(value, "resource key")
	if err != nil {
		return "", err
	}
	for i := 0; i < keySegmentCount-1; i++ {
		if parts[i] == "*" {
			return "", perrors.WithCode(code.ErrInvalidArgument, "resource key wildcard is only allowed in name segment")
		}
	}
	return Key(value), nil
}

func NewPattern(value string) (Pattern, error) {
	value = strings.TrimSpace(value)
	if _, err := parseFourSegmentResource(value, "resource pattern"); err != nil {
		return "", err
	}
	return Pattern(value), nil
}

func parseFourSegmentResource(value, label string) ([]string, error) {
	if value == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "%s is required", label)
	}
	parts := strings.Split(value, ":")
	if len(parts) != keySegmentCount {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "%s must use <app>:<domain>:<type>:<name-or-pattern>", label)
	}
	for index, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "%s contains empty segment", label)
		}
		if trimmed != part {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "%s contains untrimmed segment", label)
		}
		parts[index] = trimmed
	}
	return parts, nil
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
