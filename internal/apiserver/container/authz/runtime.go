package authz

import (
	"context"

	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
)

// PolicySyncSubscriber listens for policy version events and reloads runtime state.
type PolicySyncSubscriber interface {
	Start(context.Context) error
	Stop() error
}

// CollectRuntime copies authz background collaborators into runtime deps.
func CollectRuntime(mod *AuthzModule, subscriber cbmessaging.Subscriber, sync *PolicySyncSubscriber) {
	if mod == nil || sync == nil {
		return
	}
	*sync = mod.PolicySyncSubscriber(subscriber)
}
