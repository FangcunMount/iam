package assembler

import (
	"context"

	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/policysync"
)

func (m *AuthzModule) PolicySyncSubscriber(subscriber cbmessaging.Subscriber) *policySyncSubscriber {
	if m == nil || subscriber == nil || m.policyReloader == nil {
		return nil
	}
	recorder, _ := m.runtimeHealth.(policysync.PolicyVersionEventRecorder)
	return &policySyncSubscriber{
		subscriber: subscriber,
		handler:    policysync.NewHandler(m.policyReloader, recorder),
	}
}

type policySyncSubscriber struct {
	subscriber cbmessaging.Subscriber
	handler    *policysync.VersionEventHandler
}

func (s *policySyncSubscriber) Start(ctx context.Context) error {
	_ = ctx
	if s == nil || s.subscriber == nil || s.handler == nil {
		return nil
	}
	return s.subscriber.Subscribe(policysync.Topic, policysync.Channel, func(ctx context.Context, msg *cbmessaging.Message) error {
		if msg == nil {
			return nil
		}
		return s.handler.Handle(ctx, msg.Payload, msg.Metadata["event_type"])
	})
}

func (s *policySyncSubscriber) Stop() error {
	if s == nil || s.subscriber == nil {
		return nil
	}
	s.subscriber.Stop()
	return nil
}
