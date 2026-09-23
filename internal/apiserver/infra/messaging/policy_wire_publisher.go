package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	"github.com/FangcunMount/reliable-messaging/message"
	"github.com/FangcunMount/reliable-messaging/transport"
)

type drainingPolicyTransport interface {
	transport.Publisher
	ReliablePublisherDrain
}

// PolicyWirePublisher retains IAM's existing component-base envelope at the
// transport boundary. The stored intent/fingerprint keeps the original business
// payload; only the borrowed transport receives the encoded wire copy.
type PolicyWirePublisher struct{ next drainingPolicyTransport }

func NewPolicyWirePublisher(next drainingPolicyTransport) (*PolicyWirePublisher, error) {
	if next == nil {
		return nil, errors.New("policy wire transport required")
	}
	return &PolicyWirePublisher{next: next}, nil
}

func (p *PolicyWirePublisher) Publish(ctx context.Context, intent message.Message) transport.Result {
	input := intent.Input()
	var payload struct {
		Version int64 `json:"version"`
	}
	if !intent.Valid() || input.Producer != "iam" || input.EventType != eventing.AuthzVersionChanged || input.SchemaVersion != "v2" || input.Scope != "scope:global" || input.ContentType != "application/json" || json.Unmarshal(input.Payload, &payload) != nil || payload.Version <= 0 {
		return transport.Result{Outcome: transport.Rejected}
	}
	envelope := cbmessaging.NewMessage(input.ID, input.Payload)
	envelope.Metadata["event_type"] = input.EventType
	envelope.Metadata["aggregate_type"] = "PolicyVersion"
	envelope.Metadata["aggregate_id"] = strconv.FormatInt(payload.Version, 10)
	envelope.Metadata["source"] = "iam-outbox-relay"
	wire, err := cbmessaging.EncodeMessagePayload(envelope)
	if err != nil {
		return transport.Result{Outcome: transport.Rejected}
	}
	input.Payload = wire
	encoded, err := message.New(input)
	if err != nil {
		return transport.Result{Outcome: transport.Rejected}
	}
	return p.next.Publish(ctx, encoded)
}

func (p *PolicyWirePublisher) Drain(ctx context.Context) error { return p.next.Drain(ctx) }
