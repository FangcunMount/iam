package policy

import (
	"strconv"

	"github.com/FangcunMount/iam/internal/pkg/event"
	"github.com/FangcunMount/iam/internal/pkg/eventcatalog"
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
			eventcatalog.AuthzVersionChanged,
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
