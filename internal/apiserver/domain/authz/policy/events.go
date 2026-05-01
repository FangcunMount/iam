package policy

import (
	"strconv"

	"github.com/FangcunMount/iam/v2/internal/apiserver/eventing"
	"github.com/FangcunMount/iam/v2/pkg/event"
)

type VersionChangedPayload struct {
	TenantID string `json:"tenant_id"`
	Version  int64  `json:"version"`
}

type VersionChangedEvent struct {
	event.BaseEvent
	payload VersionChangedPayload
}

func NewVersionChangedEvent(tenantID string, version int64) VersionChangedEvent {
	return VersionChangedEvent{
		BaseEvent: event.NewBaseEvent(
			eventing.AuthzVersionChanged,
			"PolicyVersion",
			tenantID+":"+strconv.FormatInt(version, 10),
		),
		payload: VersionChangedPayload{
			TenantID: tenantID,
			Version:  version,
		},
	}
}

func (e VersionChangedEvent) Payload() any {
	return e.payload
}
