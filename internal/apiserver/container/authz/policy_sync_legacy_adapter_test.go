//go:build reliable_messaging

package authz

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policypublication"
	sdktransport "github.com/FangcunMount/reliable-messaging/transport"
	sdknsq "github.com/FangcunMount/reliable-messaging/transport/nsq"
	"github.com/FangcunMount/reliable-messaging/wire/legacy"
)

// Historical SDK contracts retain the old fanout and failure channel in this
// test-only adapter. The production factory accepts only RocketMQ subscribers.
type legacyPolicySubscriber struct {
	subscriber  *sdknsq.Subscriber
	provisioner *sdknsq.Provisioner
	failed      func(context.Context, legacy.FailedHandoff) error
}

func (s legacyPolicySubscriber) Subscribe(ctx context.Context, topic, channel string, handler sdktransport.Handler, _ func(context.Context, sdktransport.Failure) error) error {
	if err := s.provisioner.EnsureTopic(ctx, topic); err != nil {
		return err
	}
	if err := s.provisioner.EnsureChannel(ctx, legacy.FailedHandoffTopicForGroup(policypublication.Topic, ChannelPrefix), legacy.FailedHandoffChannel); err != nil {
		return err
	}
	return s.subscriber.Subscribe(ctx, topic, channel, handler, s.failed)
}

func (s legacyPolicySubscriber) Ready(time.Duration) bool        { return true }
func (s legacyPolicySubscriber) Close(ctx context.Context) error { return s.subscriber.Close(ctx) }

func (m *AuthzModule) SDKPolicySyncSubscriber(subscriber *sdknsq.Subscriber, provisioner *sdknsq.Provisioner, failed func(context.Context, legacy.FailedHandoff) error) *policySyncSubscriber {
	if subscriber == nil || provisioner == nil || failed == nil {
		return nil
	}
	return m.newPolicySyncSubscriber(legacyPolicySubscriber{subscriber, provisioner, failed}, func(context.Context, sdktransport.Failure) error { return nil }, CurrentInstanceChannel())
}
