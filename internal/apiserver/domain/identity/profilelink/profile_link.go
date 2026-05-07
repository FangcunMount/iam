package profilelink

import (
	"sync"
	"time"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ProfileLink 表示 User 与 Profile 之间的一条档案关系事实。
type ProfileLink struct {
	mu sync.RWMutex // 读写锁，保护 RevokedAt 等可变状态

	ID      meta.ID
	User    meta.ID
	Profile meta.ID

	Type          Type       // 关联类型
	Rel           Relation   // 关联关系
	EstablishedAt time.Time  // 建立时间
	RevokedAt     *time.Time // 撤销时间；nil 表示当前仍有效
}

// IsActive 判断当前档案关系是否仍然有效
func (pl *ProfileLink) IsActive() bool {
	// 读锁
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	return pl.RevokedAt == nil
}

// Revoke 撤销档案关系
// 撤销是幂等操作：如果已经撤销，则保留首次撤销时间
func (pl *ProfileLink) Revoke(at time.Time) {
	// 写锁
	pl.mu.Lock()
	defer pl.mu.Unlock()

	// 已经撤销，则保留首次撤销时间
	if pl.RevokedAt != nil {
		return
	}

	revokedAt := at
	pl.RevokedAt = &revokedAt
}
