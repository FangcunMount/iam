package sms

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/FangcunMount/iam/pkg/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginOTPSMSPayload_JSON(t *testing.T) {
	b, err := json.Marshal(LoginOTPSMSPayload{
		EventType: EventLoginOTPSMS,
		Scene:     "login",
		PhoneE164: "+8613800138000",
		Code:      "123456",
	})
	require.NoError(t, err)
	assert.Contains(t, string(b), "iam.login_otp_sms")
	assert.Contains(t, string(b), "+8613800138000")
}

func TestMQLoginOTPSenderWithPublisherPublishesLoginEvent(t *testing.T) {
	publisher := &smsPublisherStub{}
	sender := NewMQLoginOTPSenderWithPublisher(publisher)

	require.NoError(t, sender.SendLoginOTP(context.Background(), "+8613800138000", "123456"))

	require.Len(t, publisher.events, 1)
	require.Equal(t, EventLoginOTPSMS, publisher.events[0].Payload().(LoginOTPSMSPayload).EventType)
}

func TestMQLoginOTPSenderWithPublisherPropagatesPublishError(t *testing.T) {
	sender := NewMQLoginOTPSenderWithPublisher(&smsPublisherStub{err: errors.New("mq down")})

	err := sender.SendLoginOTP(context.Background(), "+8613800138000", "123456")

	require.ErrorContains(t, err, "mq down")
}

type smsPublisherStub struct {
	err    error
	events []event.DomainEvent
}

func (p *smsPublisherStub) Publish(_ context.Context, evt event.DomainEvent) error {
	if evt != nil {
		p.events = append(p.events, evt)
	}
	return p.err
}

func (p *smsPublisherStub) PublishAll(ctx context.Context, events []event.DomainEvent) error {
	for _, evt := range events {
		if err := p.Publish(ctx, evt); err != nil {
			return err
		}
	}
	return nil
}
