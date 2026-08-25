package resource

import (
	"regexp"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

var concreteActionPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Action identifies an operation in an authorization request or policy fact.
type Action string

func NewAction(value string) (Action, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", perrors.WithCode(code.ErrInvalidArgument, "action is required")
	}
	if !concreteActionPattern.MatchString(value) {
		return "", perrors.WithCode(code.ErrInvalidArgument, "action must be a concrete operation")
	}
	return Action(value), nil
}

func (a Action) String() string {
	return string(a)
}

// ActionPattern identifies an operation expression stored in authorization facts.
type ActionPattern string

func NewActionPattern(value string) (ActionPattern, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", perrors.WithCode(code.ErrInvalidArgument, "action pattern is required")
	}
	if value == "*" {
		return ActionPattern(value), nil
	}
	concrete, err := NewAction(value)
	if err != nil {
		return "", err
	}
	return ActionPattern(concrete), nil
}

func (p ActionPattern) String() string {
	return string(p)
}

// Matches reports whether this policy action pattern covers the concrete action.
func (p ActionPattern) Matches(action Action) bool {
	pattern := strings.TrimSpace(p.String())
	concrete := strings.TrimSpace(action.String())
	if pattern == "" || concrete == "" {
		return false
	}
	return pattern == "*" || pattern == concrete
}
