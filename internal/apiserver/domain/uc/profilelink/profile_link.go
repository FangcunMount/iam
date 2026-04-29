package profilelink

import (
	"sync"
	"time"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

type Relation string // 档案关系
const (
	RelSelf        Relation = "self"        // 自己
	RelParent      Relation = "parent"      // 父母
	RelGrandparent Relation = "grandparent" // 祖父母
	RelOther       Relation = "other"       // 其他
)

// Type 描述关系边的主类别，Relation 描述同一类别下的业务关系。
type Type string

const (
	TypeSelf     Type = "self"
	TypeRelation Type = "relation"
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

// NewSelfProfileLink 创建 User 与本人档案之间的强制关系。
func NewSelfProfileLink(userID meta.ID, profileID meta.ID, now time.Time) *ProfileLink {
	return &ProfileLink{
		User:          userID,
		Profile:       profileID,
		Type:          TypeSelf,
		Rel:           RelSelf,
		EstablishedAt: now,
	}
}

func TypeFromRelation(relation Relation) Type {
	if relation == RelSelf {
		return TypeSelf
	}
	return TypeRelation
}
