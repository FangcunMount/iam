package eventing

import (
	"encoding/json"
	"errors"
	"github.com/FangcunMount/iam/v5/internal/pkg/timezone"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/FangcunMount/iam/v5/pkg/eventcodec"
	"github.com/FangcunMount/reliable-messaging/message"
	"strconv"
	"time"
)

// PolicyIntentMapper preserves the immutable policy notification format. It
// owns no database, publication queue, transport or background work.
type PolicyIntentMapper struct{ topic string }

func NewPolicyIntentMapper(catalog *eventcatalog.Catalog) (*PolicyIntentMapper, error) {
	if catalog == nil {
		return nil, errors.New("policy event catalog required")
	}
	topic, ok := catalog.GetTopicForEvent(AuthzVersionChanged)
	if !ok || topic == "" || !catalog.IsDurableOutbox(AuthzVersionChanged) {
		return nil, errors.New("durable policy route required")
	}
	return &PolicyIntentMapper{topic: topic}, nil
}

func (s *PolicyIntentMapper) Message(evt event.DomainEvent) (message.Message, error) {
	if evt == nil || evt.EventType() != AuthzVersionChanged || evt.AggregateType() != "PolicyVersion" || evt.OccurredAt().IsZero() {
		return message.Message{}, errors.New("unsupported policy event")
	}
	payload, err := eventcodec.EncodePayload(evt)
	if err != nil {
		return message.Message{}, err
	}
	version, err := PolicyPayloadVersion(payload)
	if err != nil || evt.AggregateID() != strconv.FormatInt(version, 10) {
		return message.Message{}, errors.New("invalid policy version payload")
	}
	return message.New(message.Input{
		Producer: "iam", ID: evt.EventID(), Destination: s.topic,
		EventType: evt.EventType(), SchemaVersion: "v2", Scope: "scope:global",
		ContentType: "application/json", OccurredAt: evt.OccurredAt().In(timezone.Location).Format(time.RFC3339Nano), Payload: payload,
	})
}

func PolicyPayloadVersion(payload []byte) (int64, error) {
	var data struct {
		Version int64 `json:"version"`
	}
	if err := json.Unmarshal(payload, &data); err != nil || data.Version <= 0 {
		return 0, errors.New("invalid policy version payload")
	}
	return data.Version, nil
}
