package loginidentity

import (
	"context"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// UnlinkOutcome 是原子解绑动作在同一事务内核对归属、活跃身份数量并解绑后的结果。
type UnlinkOutcome uint8

const (
	UnlinkOutcomeUnlinked UnlinkOutcome = iota + 1
	UnlinkOutcomeNotFound
	UnlinkOutcomeLastActive
)

// AtomicIdentityUnlinker 保证“至少保留一个活跃身份”与状态更新不可被并发穿透。
type AtomicIdentityUnlinker interface {
	UnlinkOwnedUnlessLastActive(ctx context.Context, userID, loginIdentityID meta.ID) (UnlinkOutcome, error)
}
