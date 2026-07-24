package sessionrevocation

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

const (
	ActionRevokeAll = "revoke_all"

	ReasonUserBlocked     = "user_blocked"
	ReasonUserDeactivated = "user_deactivated"
)

// Stager writes a durable revocation task inside the caller's Identity
// transaction. Implementations resolve the committed user version locally.
type Stager interface {
	Stage(ctx context.Context, userID meta.ID, action, reason string) error
}
