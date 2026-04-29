package profilelink

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/internal/apiserver/domain/uc/profile"
	"github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
)

// SelfProfileEnsurer ensures every login user has exactly one active self profile link.
type SelfProfileEnsurer struct {
	profiles ProfileCreator
	links    Repository
	now      func() time.Time
}

// ProfileCreator is the profile persistence capability required by SelfProfileEnsurer.
type ProfileCreator interface {
	Create(ctx context.Context, profile *profile.Profile) error
}

// NewSelfProfileEnsurer creates the self profile invariant enforcer.
func NewSelfProfileEnsurer(profiles ProfileCreator, links Repository) *SelfProfileEnsurer {
	return &SelfProfileEnsurer{
		profiles: profiles,
		links:    links,
		now:      time.Now,
	}
}

// Ensure creates a self Profile and ProfileLink when the user has no active self link.
func (e *SelfProfileEnsurer) Ensure(ctx context.Context, u *user.User) error {
	if e == nil || u == nil || e.profiles == nil || e.links == nil {
		return nil
	}

	existingLinks, err := e.links.FindByUserID(ctx, u.ID)
	if err != nil {
		return err
	}
	for _, existing := range existingLinks {
		if existing != nil && existing.Type == TypeSelf && existing.IsActive() {
			return nil
		}
	}

	selfProfile, err := profile.NewProfile(u.Name)
	if err != nil {
		return err
	}
	if err := e.profiles.Create(ctx, selfProfile); err != nil {
		return err
	}
	return e.links.Create(ctx, NewSelfProfileLink(u.ID, selfProfile.ID, e.now()))
}
