package authz

import (
	"context"

	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policysync"
)

func (m *AuthzModule) PolicySyncSubscriber(subscriber cbmessaging.Subscriber) *policySyncSubscriber {
	if m == nil || subscriber == nil || m.policyReloader == nil {
		return nil
	}
	recorder, _ := m.runtimeHealth.(policysync.PolicyVersionEventRecorder)
	channel := policysync.CurrentInstanceChannel()
	if setter, ok := m.runtimeHealth.(interface{ SetPolicySyncChannel(string) }); ok {
		setter.SetPolicySyncChannel(channel)
	}
	return &policySyncSubscriber{
		subscriber: subscriber,
		handler:    policysync.NewHandler(m.policyReloader, recorder),
		channel:    channel,
	}
}

type policySyncSubscriber struct {
	subscriber cbmessaging.Subscriber
	handler    *policysync.VersionEventHandler
	channel    string
}

func (s *policySyncSubscriber) Start(ctx context.Context) error {
	_ = ctx
	if s == nil || s.subscriber == nil || s.handler == nil {
		return nil
	}
	channel := s.Channel()
	return s.subscriber.Subscribe(policysync.Topic, channel, func(ctx context.Context, msg *cbmessaging.Message) error {
		if msg == nil {
			return nil
		}
		return s.handler.Handle(ctx, msg.Payload, msg.Metadata["event_type"])
	})
}

func (s *policySyncSubscriber) Channel() string {
	if s == nil || s.channel == "" {
		return policysync.CurrentInstanceChannel()
	}
	return s.channel
}

func (s *policySyncSubscriber) Stop() error {
	if s == nil || s.subscriber == nil {
		return nil
	}
	s.subscriber.Stop()
	return nil
}
