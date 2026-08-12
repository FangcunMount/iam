package scope

import (
	"fmt"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// Kind identifies the object range covered by a permission or request.
type Kind string

const (
	KindAll    Kind = "all"
	KindOrigin Kind = "origin"
)

const defaultValue = "*"

// Scope describes the object range for an authorization fact.
type Scope struct {
	Kind  Kind
	Value string
}

func Default() Scope {
	return Scope{Kind: KindAll, Value: defaultValue}
}

func New(kind Kind, value string) (Scope, error) {
	kind = Kind(strings.TrimSpace(string(kind)))
	value = strings.TrimSpace(value)
	if kind == "" {
		if value != "" {
			return Scope{}, perrors.WithCode(code.ErrInvalidArgument, "scope kind is required when scope value is provided")
		}
		return Default(), nil
	}
	switch kind {
	case KindAll:
		if value == "" {
			value = defaultValue
		}
		if value != defaultValue {
			return Scope{}, perrors.WithCode(code.ErrInvalidArgument, "all scope value must be *")
		}
	case KindOrigin:
		if value == "" || value == defaultValue {
			return Scope{}, perrors.WithCode(code.ErrInvalidArgument, "origin scope value is required")
		}
	default:
		return Scope{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported scope kind: %s", kind)
	}
	return Scope{Kind: kind, Value: value}, nil
}

func Normalize(kind, value string) (Scope, error) {
	return New(Kind(kind), value)
}

func Parse(encoded string) (Scope, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return Default(), nil
	}
	parts := strings.SplitN(encoded, ":", 2)
	if len(parts) != 2 {
		return Scope{}, perrors.WithCode(code.ErrInvalidArgument, "invalid scope format")
	}
	return New(Kind(parts[0]), parts[1])
}

func (s Scope) IsZero() bool {
	return s.Kind == "" && s.Value == ""
}

func (s Scope) Normalized() Scope {
	if s.IsZero() {
		return Default()
	}
	return s
}

func (s Scope) String() string {
	normalized := s.Normalized()
	return fmt.Sprintf("%s:%s", normalized.Kind, normalized.Value)
}
