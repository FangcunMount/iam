package eventoutbox

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	dbmysql "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	"github.com/FangcunMount/iam/v5/internal/pkg/timezone"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"github.com/FangcunMount/iam/v5/pkg/eventcodec"
	"github.com/FangcunMount/reliable-messaging/message"
	sdkmysql "github.com/FangcunMount/reliable-messaging/storage/mysql"
)

// StandardStager maps IAM's policy event to the shared schema. It never reads or
// writes the historical Outbox and delegates identity conflict handling to SDK.
type StandardStager struct{ topic string }

var _ event.Stager = (*StandardStager)(nil)

func NewStandardStager(catalog *eventcatalog.Catalog) (*StandardStager, error) {
	if catalog == nil {
		return nil, errors.New("policy event catalog required")
	}
	topic, ok := catalog.GetTopicForEvent(eventing.AuthzVersionChanged)
	if !ok || topic == "" || !catalog.IsDurableOutbox(eventing.AuthzVersionChanged) {
		return nil, errors.New("durable policy route required")
	}
	return &StandardStager{topic: topic}, nil
}

func (s *StandardStager) Stage(ctx context.Context, events ...event.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := dbmysql.RequireTx(ctx)
	if err != nil {
		return err
	}
	if tx.Dialector == nil || tx.Dialector.Name() != "mysql" {
		return errors.New("standard reliable Stage requires MySQL")
	}
	appender, err := sdkmysql.BindGORM(tx)
	if err != nil {
		return err
	}
	for _, evt := range events {
		intent, err := s.intent(evt)
		if err != nil {
			return err
		}
		if err = appender.Append(ctx, intent, time.Now()); err != nil {
			return err
		}
	}
	return nil
}

func (s *StandardStager) intent(evt event.DomainEvent) (message.Message, error) {
	if evt == nil || evt.EventType() != eventing.AuthzVersionChanged || evt.AggregateType() != "PolicyVersion" || evt.OccurredAt().IsZero() {
		return message.Message{}, errors.New("unsupported policy event")
	}
	payload, err := eventcodec.EncodePayload(evt)
	if err != nil {
		return message.Message{}, err
	}
	version, err := policyPayloadVersion(payload)
	if err != nil || evt.AggregateID() != strconv.FormatInt(version, 10) {
		return message.Message{}, errors.New("invalid policy version payload")
	}
	return message.New(message.Input{
		Producer: "iam", ID: evt.EventID(), Destination: s.topic,
		EventType: evt.EventType(), SchemaVersion: "v2", Scope: "scope:global",
		ContentType: "application/json", OccurredAt: evt.OccurredAt().In(timezone.Location).Format(time.RFC3339Nano), Payload: payload,
	})
}

func policyPayloadVersion(payload []byte) (int64, error) {
	var data struct {
		Version int64 `json:"version"`
	}
	if err := json.Unmarshal(payload, &data); err != nil || data.Version <= 0 {
		return 0, errors.New("invalid policy version payload")
	}
	return data.Version, nil
}
