package eventcodec

import (
	"encoding/json"
	"fmt"

	"github.com/FangcunMount/iam/v2/pkg/event"
)

const occurredAtLayout = "2006-01-02T15:04:05.000Z07:00"

// EncodePayload serializes the event wire payload.
func EncodePayload(evt event.DomainEvent) ([]byte, error) {
	if evt == nil {
		return nil, fmt.Errorf("domain event is nil")
	}
	payload, err := json.Marshal(evt.Payload())
	if err != nil {
		return nil, fmt.Errorf("marshal event payload: %w", err)
	}
	return payload, nil
}

func MetadataFromEvent(evt event.DomainEvent, source string) map[string]string {
	if evt == nil {
		return map[string]string{}
	}
	return map[string]string{
		"event_type":     evt.EventType(),
		"aggregate_type": evt.AggregateType(),
		"aggregate_id":   evt.AggregateID(),
		"occurred_at":    evt.OccurredAt().Format(occurredAtLayout),
		"source":         source,
	}
}
