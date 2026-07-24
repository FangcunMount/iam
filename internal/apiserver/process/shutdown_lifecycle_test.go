package process

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/FangcunMount/component-base/pkg/processruntime"
)

func TestRunShutdownSequenceKeepsLifecycleAndCloseOrder(t *testing.T) {
	var order []string
	var lifecycle processruntime.Lifecycle
	lifecycle.AddShutdownHook("stop outbox relay", func() error {
		order = append(order, "stop outbox relay")
		return nil
	})
	lifecycle.AddShutdownHook("stop key rotation scheduler", func() error {
		order = append(order, "stop key rotation scheduler")
		return nil
	})

	runShutdownSequence(shutdownSequenceDeps{
		lifecycle: lifecycle,
		beginDrain: func() {
			order = append(order, "begin drain")
		},
		drainDelay: time.Second,
		waitDrain: func(time.Duration) {
			order = append(order, "drain delay")
		},
		suggestCleanup: func() error {
			order = append(order, "suggest cleanup")
			return nil
		},
		identityCleanup: func() error {
			order = append(order, "identity cleanup")
			return nil
		},
		closeDatabase: func() error {
			order = append(order, "database close")
			return nil
		},
		closeHTTP: func() error {
			order = append(order, "http close")
			return nil
		},
		closeGRPC: func() error {
			order = append(order, "grpc close")
			return nil
		},
	})

	want := []string{
		"begin drain",
		"drain delay",
		"stop outbox relay",
		"stop key rotation scheduler",
		"suggest cleanup",
		"identity cleanup",
		"database close",
		"http close",
		"grpc close",
	}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("shutdown order = %#v, want %#v", order, want)
	}
}

func TestStartAuthzPolicySyncRegistersShutdownHook(t *testing.T) {
	var lifecycle processruntime.Lifecycle
	sync := &authzPolicySyncFake{}

	startAuthzPolicySync(&lifecycle, sync)

	if sync.started != 1 {
		t.Fatalf("start count = %d, want 1", sync.started)
	}
	if lifecycle.Len() != 1 {
		t.Fatalf("shutdown hooks = %d, want 1", lifecycle.Len())
	}
	lifecycle.Run(func(string, error) {})
	if sync.stopped != 1 {
		t.Fatalf("stop count = %d, want 1", sync.stopped)
	}
}

func TestStartAuthzPolicySyncDoesNotRegisterHookWhenStartFails(t *testing.T) {
	var lifecycle processruntime.Lifecycle
	sync := &authzPolicySyncFake{startErr: errors.New("boom")}

	startAuthzPolicySync(&lifecycle, sync)

	if sync.started != 1 {
		t.Fatalf("start count = %d, want 1", sync.started)
	}
	if lifecycle.Len() != 0 {
		t.Fatalf("shutdown hooks = %d, want 0", lifecycle.Len())
	}
}

type authzPolicySyncFake struct {
	startErr error
	started  int
	stopped  int
}

func (f *authzPolicySyncFake) Start(context.Context) error {
	f.started++
	return f.startErr
}

func (f *authzPolicySyncFake) Stop() error {
	f.stopped++
	return nil
}
