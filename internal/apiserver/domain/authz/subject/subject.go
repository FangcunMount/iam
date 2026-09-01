package subject

import (
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Type identifies the kind of principal that can receive authorization.
type Type string

const (
	TypeUser    Type = "user"
	TypeGroup   Type = "group"
	TypeService Type = "service"
)

// Ref is the business principal being authorized.
type Ref struct {
	Type Type
	ID   meta.ID
}

func NewRef(subjectType Type, id meta.ID) (Ref, error) {
	subjectType = Type(strings.TrimSpace(string(subjectType)))
	if subjectType == "" {
		return Ref{}, perrors.WithCode(code.ErrInvalidArgument, "subject type is required")
	}
	switch subjectType {
	case TypeUser, TypeGroup, TypeService:
	default:
		return Ref{}, perrors.WithCode(code.ErrInvalidArgument, "unsupported subject type: %s", subjectType)
	}
	if id.IsZero() {
		return Ref{}, perrors.WithCode(code.ErrInvalidArgument, "subject id is required")
	}
	return Ref{Type: subjectType, ID: id}, nil
}

func NewUserRef(id meta.ID) (Ref, error) {
	return NewRef(TypeUser, id)
}

// ParseRef parses the canonical <type>:<id> representation of a subject.
func ParseRef(value string) (Ref, error) {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Ref{}, perrors.WithCode(code.ErrInvalidArgument, "subject must use <type>:<id>")
	}
	id, err := meta.ParseID(parts[1])
	if err != nil {
		return Ref{}, perrors.WithCode(code.ErrInvalidArgument, "subject id is invalid")
	}
	return NewRef(Type(parts[0]), id)
}

func (r Ref) IsZero() bool {
	return r.Type == "" || r.ID.IsZero()
}

func (r Ref) String() string {
	if r.IsZero() {
		return ""
	}
	return string(r.Type) + ":" + r.ID.String()
}
