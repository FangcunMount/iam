package authz

import "context"

// PolicySyncSubscriber listens for policy version events and reloads runtime state.
type PolicySyncSubscriber interface {
	Start(context.Context) error
	Stop() error
}
