package profilelink

import (
	"sync"
	"time"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ProfileLink 档案关系
type ProfileLink struct {
	mu            sync.RWMutex `json:"-"`
	ID            meta.ID
	User          meta.ID
	Profile       meta.ID
	Type          Type
	Rel           Relation
	EstablishedAt time.Time
	RevokedAt     *time.Time
}

// IsActive 是否有效
func (g *ProfileLink) IsActive() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.RevokedAt == nil
}

// Revoke 撤销档案关系 (并发安全)
func (g *ProfileLink) Revoke(at time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	// allocate a fresh time on the heap and copy the value so
	// concurrent callers don't end up writing the same stack address
	// (the race detector can still observe races when &at is used).
	t := new(time.Time)
	*t = at
	g.RevokedAt = t
}
