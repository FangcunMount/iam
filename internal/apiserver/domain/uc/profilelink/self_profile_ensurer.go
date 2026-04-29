package profilelink

import (
	"context"
	"sort"
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
// If historical data contains multiple active self links, the earliest one is
// kept as self and the rest are converted to parent relations.
func (e *SelfProfileEnsurer) Ensure(ctx context.Context, u *user.User) error {
	if e == nil || u == nil || e.profiles == nil || e.links == nil {
		return nil
	}

	existingLinks, err := e.links.FindByUserID(ctx, u.ID)
	if err != nil {
		return err
	}
	activeSelfLinks := activeSelfProfileLinks(existingLinks)
	if len(activeSelfLinks) > 0 {
		return e.ensureSingleActiveSelfLink(ctx, activeSelfLinks)
	}

	selfProfile, err := profile.NewFromCreationSpec(profile.CreationSpec{Name: u.Name})
	if err != nil {
		return err
	}
	if err := e.profiles.Create(ctx, selfProfile); err != nil {
		return err
	}
	return e.links.Create(ctx, NewSelfProfileLink(u.ID, selfProfile.ID, e.now()))
}

func activeSelfProfileLinks(links []*ProfileLink) []*ProfileLink {
	active := make([]*ProfileLink, 0, len(links))
	for _, existing := range links {
		if existing != nil && existing.Type == TypeSelf && existing.IsActive() {
			active = append(active, existing)
		}
	}
	return active
}

func (e *SelfProfileEnsurer) ensureSingleActiveSelfLink(ctx context.Context, activeSelfLinks []*ProfileLink) error {
	sort.SliceStable(activeSelfLinks, func(i, j int) bool {
		left, right := activeSelfLinks[i], activeSelfLinks[j]
		if left.EstablishedAt.Equal(right.EstablishedAt) {
			return left.ID.Uint64() < right.ID.Uint64()
		}
		return left.EstablishedAt.Before(right.EstablishedAt)
	})

	for _, duplicate := range activeSelfLinks[1:] {
		duplicate.ConvertToRelation(RelParent)
		if err := e.links.Update(ctx, duplicate); err != nil {
			return err
		}
	}
	return nil
}
