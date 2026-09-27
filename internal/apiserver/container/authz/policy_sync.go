package authz

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policypublication"
	authzruntime "github.com/FangcunMount/iam/v5/internal/apiserver/infra/authz/runtime"
	sdktransport "github.com/FangcunMount/reliable-messaging/transport"
	sdknsq "github.com/FangcunMount/reliable-messaging/transport/nsq"
	"github.com/FangcunMount/reliable-messaging/wire/legacy"
)

type policySyncRuntime interface {
	Reconcile(context.Context) error
	SyncConfig() authzruntime.Config
	SetSyncState(bool, bool)
}

func (m *AuthzModule) PolicySyncSubscriber(subscriber cbmessaging.Subscriber) *policySyncSubscriber {
	if m == nil || subscriber == nil || m.policyReloader == nil {
		return nil
	}
	m.syncOnce.Do(func() {
		recorder, _ := m.runtimeHealth.(policypublication.PolicyVersionEventRecorder)
		runtime, _ := m.policyReloader.(policySyncRuntime)
		channel := CurrentInstanceChannel()
		if setter, ok := m.runtimeHealth.(interface{ SetPolicySyncChannel(string) }); ok {
			setter.SetPolicySyncChannel(channel)
		}
		m.policySync = &policySyncSubscriber{subscriber: subscriber, handler: policypublication.NewService(m.policyReloader, recorder), channel: channel, runtime: runtime}
	})
	return m.policySync
}

type policySyncSubscriber struct {
	subscriber       cbmessaging.Subscriber
	sdkSubscriber    *sdknsq.Subscriber
	sdkProvisioner   *sdknsq.Provisioner
	sdkFailed        func(context.Context, legacy.FailedHandoff) error
	handler          *policypublication.Service
	channel          string
	runtime          policySyncRuntime
	mu               sync.Mutex
	cancel           context.CancelFunc
	done             chan struct{}
	started, stopped bool
	registered       bool // owned by the single synchronization loop
	stopOnce         sync.Once
	sdkStopMu        sync.Mutex
	sdkClosed        bool
}

// SDKPolicySyncSubscriber retains the existing per-process business channel
// and uses a stable failure group so a replacement process can audit failures
// left by its predecessor. Only one of the legacy and SDK constructors may be
// selected for a module instance.
func (m *AuthzModule) SDKPolicySyncSubscriber(subscriber *sdknsq.Subscriber, provisioner *sdknsq.Provisioner, failed func(context.Context, legacy.FailedHandoff) error) *policySyncSubscriber {
	if m == nil || subscriber == nil || provisioner == nil || failed == nil || m.policyReloader == nil {
		return nil
	}
	m.syncOnce.Do(func() {
		recorder, _ := m.runtimeHealth.(policypublication.PolicyVersionEventRecorder)
		runtime, _ := m.policyReloader.(policySyncRuntime)
		channel := CurrentInstanceChannel()
		if setter, ok := m.runtimeHealth.(interface{ SetPolicySyncChannel(string) }); ok {
			setter.SetPolicySyncChannel(channel)
		}
		m.policySync = &policySyncSubscriber{
			sdkSubscriber: subscriber, sdkProvisioner: provisioner, sdkFailed: failed,
			handler: policypublication.NewService(m.policyReloader, recorder), channel: channel, runtime: runtime,
		}
	})
	return m.policySync
}

// Start establishes a managed loop even if the first subscription attempt fails.
// The failure is reflected in readiness and retried; the lifecycle always owns Stop.
func (s *policySyncSubscriber) Start(ctx context.Context) error {
	if s == nil || (s.subscriber == nil && s.sdkSubscriber == nil) || s.handler == nil {
		return fmt.Errorf("policy sync dependencies unavailable")
	}
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return fmt.Errorf("policy sync stopped")
	}
	if s.started {
		s.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})
	s.started = true
	s.mu.Unlock()
	interval := authzruntime.DefaultConfig().CheckInterval
	if s.runtime != nil {
		interval = s.runtime.SyncConfig().CheckInterval
		s.runtime.SetSyncState(true, false)
	}
	// Own the loop before registration; Start still waits for the initial check.
	initialized := make(chan struct{})
	go func() {
		defer close(s.done)
		defer func() {
			if s.runtime != nil {
				s.runtime.SetSyncState(false, false)
			}
		}()
		s.step(runCtx)
		close(initialized)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				s.step(runCtx)
			}
		}
	}()
	select {
	case <-initialized:
	case <-runCtx.Done():
	}
	return nil
}

func (s *policySyncSubscriber) step(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	if !s.registered {
		var err error
		if s.sdkSubscriber != nil {
			failureTopic := legacy.FailedHandoffTopicForGroup(policypublication.Topic, ChannelPrefix)
			err = s.sdkProvisioner.EnsureTopic(ctx, policypublication.Topic)
			if err == nil {
				err = s.sdkProvisioner.EnsureChannel(ctx, failureTopic, legacy.FailedHandoffChannel)
			}
			if err == nil {
				err = s.sdkSubscriber.Subscribe(ctx, policypublication.Topic, s.Channel(), func(deliveryCtx context.Context, delivery sdktransport.Delivery) error {
					message := delivery.Message()
					return s.handle(ctx, deliveryCtx, message.Payload, message.Metadata["event_type"])
				}, s.sdkFailed)
			}
		} else {
			err = s.subscriber.Subscribe(policypublication.Topic, s.Channel(), func(deliveryCtx context.Context, msg *cbmessaging.Message) error {
				if msg == nil {
					return nil
				}
				return s.handle(ctx, deliveryCtx, msg.Payload, msg.Metadata["event_type"])
			})
		}
		s.registered = err == nil
		if err != nil {
			log.Errorw("authz policy subscription failed; retrying", "error", err)
		}
	}
	if s.runtime != nil {
		s.runtime.SetSyncState(true, s.registered)
		if err := s.runtime.Reconcile(ctx); err != nil {
			log.Errorw("authz policy version reconciliation failed", "error", err)
		}
	}
}

func (s *policySyncSubscriber) handle(runCtx, deliveryCtx context.Context, payload []byte, eventType string) error {
	callbackCtx, cancel := context.WithCancel(deliveryCtx)
	defer cancel()
	stop := context.AfterFunc(runCtx, cancel)
	defer stop()
	return s.handler.Handle(callbackCtx, payload, eventType)
}
func (s *policySyncSubscriber) Channel() string {
	if s == nil {
		return ""
	}
	return s.channel
}
func (s *policySyncSubscriber) Stop() error {
	if s == nil {
		return nil
	}
	if s.sdkSubscriber != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return s.StopWithContext(ctx)
	}
	s.stopOnce.Do(func() {
		s.mu.Lock()
		s.stopped = true
		cancel, done := s.cancel, s.done
		s.mu.Unlock()
		if cancel != nil {
			cancel()
			<-done
		}
		if s.subscriber != nil {
			s.subscriber.Stop()
		}
	})
	return nil
}

// StopWithContext is a retryable resource-close gate for the SDK path. A
// timeout leaves IAM's MySQL connection open for the next drain attempt.
func (s *policySyncSubscriber) StopWithContext(ctx context.Context) error {
	if s == nil || s.sdkSubscriber == nil {
		return s.Stop()
	}
	s.sdkStopMu.Lock()
	defer s.sdkStopMu.Unlock()
	if s.sdkClosed {
		return nil
	}
	s.mu.Lock()
	s.stopped = true
	cancel, done := s.cancel, s.done
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := s.sdkSubscriber.Close(ctx); err != nil {
		return err
	}
	s.sdkClosed = true
	return nil
}
