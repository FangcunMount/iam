package guardianship

import (
	"sync"
	"time"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

type Relation string // 监护关系
const (
	RelSelf        Relation = "self"        // 自己
	RelParent      Relation = "parent"      // 父母
	RelGrandparent Relation = "grandparent" // 祖父母
	RelOther       Relation = "other"       // 其他
)

// Guardianship 监护关系
type Guardianship struct {
	mu            sync.RWMutex `json:"-"`
	ID            meta.ID
	User          meta.ID
	Child         meta.ID
	Rel           Relation
	EstablishedAt time.Time
	RevokedAt     *time.Time
}

// IsActive 是否有效
func (g *Guardianship) IsActive() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.RevokedAt == nil
}

// Revoke 撤销监护关系 (并发安全)
func (g *Guardianship) Revoke(at time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	// allocate a fresh time on the heap and copy the value so
	// concurrent callers don't end up writing the same stack address
	// (the race detector can still observe races when &at is used).
	t := new(time.Time)
	*t = at
	g.RevokedAt = t
}
