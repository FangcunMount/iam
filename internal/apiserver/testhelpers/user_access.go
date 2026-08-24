package testhelpers

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// UserResolverStub is a narrow Identity capability stub for sibling-module tests.
type UserResolverStub struct {
	Existing map[meta.ID]bool
	Err      error
}

func NewUserResolverStub(ids ...meta.ID) *UserResolverStub {
	existing := make(map[meta.ID]bool, len(ids))
	for _, id := range ids {
		existing[id] = true
	}
	return &UserResolverStub{Existing: existing}
}

func (s *UserResolverStub) ResolveUser(_ context.Context, userID meta.ID) error {
	if s != nil && s.Err != nil {
		return s.Err
	}
	if s == nil || !s.Existing[userID] {
		return perrors.WithCode(code.ErrUserNotFound, "user not found")
	}
	return nil
}
