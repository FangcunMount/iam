package authz

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policypublication"
	authzruntime "github.com/FangcunMount/iam/v5/internal/apiserver/infra/authz/runtime"
	sdktransport "github.com/FangcunMount/reliable-messaging/transport"
	"github.com/FangcunMount/reliable-messaging/transport/rocketmq"
)

type policySyncRuntime interface {
	Reconcile(context.Context) error
	SyncConfig() authzruntime.Config
	SetSyncState(bool, bool)
}

// Narrow host lifecycle seams use SDK delivery contracts; transport algorithms remain in the SDK.
type policySubscriber interface {
	Subscribe(context.Context, string, string, sdktransport.Handler, func(context.Context, sdktransport.Failure) error) error
	Ready(time.Duration) bool
	Close(context.Context) error
}

type policySyncSubscriber struct {
	subscriber       policySubscriber
	failed           func(context.Context, sdktransport.Failure) error
	handler          *policypublication.Service
	channel          string
	runtime          policySyncRuntime
	mu               sync.Mutex
	cancel           context.CancelFunc
	done             chan struct{}
	started, stopped bool
	registered       bool // owned by the single synchronization loop
	sdkStopMu        sync.Mutex
	sdkClosed        bool
}

// Start establishes a managed loop even if the first subscription attempt fails.
// The failure is reflected in readiness and retried; the lifecycle always owns Stop.
func (s *policySyncSubscriber) Start(ctx context.Context) error {
	if s == nil || s.subscriber == nil || s.handler == nil {
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
		err := s.subscriber.Subscribe(ctx, policypublication.Topic, s.Channel(), func(deliveryCtx context.Context, delivery sdktransport.Delivery) error {
			m := delivery.Message()
			return s.handle(ctx, deliveryCtx, m.Payload, m.Metadata["event_type"])
		}, s.failed)
		s.registered = err == nil
		if err != nil {
			log.Errorw("authz policy subscription failed; retrying", "error", err)
		}
	}
	if s.runtime != nil {
		s.runtime.SetSyncState(true, s.registered && s.subscriber.Ready(30*time.Second))
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.StopWithContext(ctx)
}

// StopWithContext is a retryable resource-close gate for the SDK path. A
// timeout leaves IAM's MySQL connection open for the next drain attempt.
func (s *policySyncSubscriber) StopWithContext(ctx context.Context) error {
	if s == nil || s.subscriber == nil {
		return nil
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
	if err := s.subscriber.Close(ctx); err != nil {
		return err
	}

	s.sdkClosed = true
	return nil
}

// RocketMQPolicySyncSubscriber uses a stable per-instance fanout subscription.
// Topology and ACLs are provisioned by infra, never silently created here.
func (m *AuthzModule) RocketMQPolicySyncSubscriber(subscriber *rocketmq.Subscriber, failed func(context.Context, sdktransport.Failure) error, channel string) *policySyncSubscriber {
	if subscriber == nil {
		return nil
	}
	return m.newPolicySyncSubscriber(subscriber, failed, channel)
}

func (m *AuthzModule) newPolicySyncSubscriber(subscriber policySubscriber, failed func(context.Context, sdktransport.Failure) error, channel string) *policySyncSubscriber {
	if m == nil || subscriber == nil || failed == nil || m.policyReloader == nil || channel == "" {
		return nil
	}
	m.syncOnce.Do(func() {
		recorder, _ := m.runtimeHealth.(policypublication.PolicyVersionEventRecorder)
		runtime, _ := m.policyReloader.(policySyncRuntime)
		if setter, ok := m.runtimeHealth.(interface{ SetPolicySyncChannel(string) }); ok {
			setter.SetPolicySyncChannel(channel)
		}
		m.policySync = &policySyncSubscriber{subscriber: subscriber, failed: failed, handler: policypublication.NewService(m.policyReloader, recorder), channel: channel, runtime: runtime}
	})
	return m.policySync
}
