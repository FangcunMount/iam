// Package useraccess defines the narrow Identity capabilities exposed to
// authentication and authorization modules. Consumers depend on these facts,
// not on the User aggregate repository.
package useraccess

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	userdomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusBlocked  Status = "blocked"
	StatusMissing  Status = "missing"
)

// UserStatusReader exposes only the user lifecycle fact required by AuthN.
type UserStatusReader interface {
	ReadUserStatus(ctx context.Context, userID meta.ID) (Status, error)
}

// UserResolver validates that a stable Identity User anchor exists.
type UserResolver interface {
	ResolveUser(ctx context.Context, userID meta.ID) error
}

// Service adapts the Identity User repository to its published narrow capabilities.
type Service struct {
	users userdomain.Repository
}

func NewService(users userdomain.Repository) *Service {
	return &Service{users: users}
}

func (s *Service) ReadUserStatus(ctx context.Context, userID meta.ID) (Status, error) {
	user, err := s.findUser(ctx, userID)
	if err != nil {
		if perrors.IsCode(err, code.ErrUserNotFound) {
			return StatusMissing, nil
		}
		return "", err
	}
	switch {
	case user.IsBlocked():
		return StatusBlocked, nil
	case user.IsInactive():
		return StatusInactive, nil
	default:
		return StatusActive, nil
	}
}

func (s *Service) ResolveUser(ctx context.Context, userID meta.ID) error {
	_, err := s.findUser(ctx, userID)
	return err
}

func (s *Service) findUser(ctx context.Context, userID meta.ID) (*userdomain.User, error) {
	if s == nil || s.users == nil {
		return nil, perrors.WithCode(code.ErrInternalServerError, "identity user capability is not configured")
	}
	if userID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "user id is required")
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, perrors.WithCode(code.ErrUserNotFound, "user(%s) not found", userID.String())
	}
	return user, nil
}
