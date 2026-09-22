package eventoutbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	"github.com/FangcunMount/reliable-messaging/message"
)

// policyIntent is the explicit historical mapping for the first SDK flow. It
// never rewrites the stored payload or invents identities for unknown rows.
// expectedTopic is resolved by the host catalog, not supplied by the message.
// The original row creation time is the stable historical time surrogate;
// historical rows do not persist the original event OccurredAt separately.
func policyIntent(row OutboxPO, expectedTopic string) (message.Message, error) {
	if row.EventType != eventing.AuthzVersionChanged || row.AggregateType != "PolicyVersion" || expectedTopic == "" || row.TopicName != expectedTopic {
		return message.Message{}, errors.New("unsupported historical outbox mapping")
	}
	var payload struct {
		Version int64 `json:"version"`
	}
	if err := json.Unmarshal([]byte(row.PayloadJSON), &payload); err != nil || payload.Version <= 0 {
		return message.Message{}, errors.New("invalid historical policy version payload")
	}
	if row.AggregateID != strconv.FormatInt(payload.Version, 10) {
		return message.Message{}, errors.New("historical aggregate and payload version differ")
	}
	if row.CreatedAt.IsZero() {
		return message.Message{}, errors.New("historical creation time missing")
	}
	// DATETIME stores clock digits, not an offset. Canonicalize those persisted
	// digits independently of the driver's loc setting. The Z suffix is only a
	// stable SDK time surrogate; it is not evidence of the real event's timezone.
	created := row.CreatedAt
	canonicalTime := time.Date(created.Year(), created.Month(), created.Day(), created.Hour(), created.Minute(), created.Second(), created.Nanosecond(), time.UTC)
	intent, err := message.New(message.Input{
		Producer: "iam", ID: row.EventID, Destination: row.TopicName,
		EventType: row.EventType, SchemaVersion: "v2", Scope: "scope:global",
		ContentType: "application/json", OccurredAt: canonicalTime.Format(time.RFC3339Nano), Payload: []byte(row.PayloadJSON),
	})
	if err != nil {
		return message.Message{}, fmt.Errorf("historical policy intent: %w", err)
	}
	return intent, nil
}
