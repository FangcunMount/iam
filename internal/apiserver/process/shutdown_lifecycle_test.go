package process

import (
	"reflect"
	"testing"

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
		suggestCleanup: func() error {
			order = append(order, "suggest cleanup")
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
		"stop outbox relay",
		"stop key rotation scheduler",
		"suggest cleanup",
		"database close",
		"http close",
		"grpc close",
	}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("shutdown order = %#v, want %#v", order, want)
	}
}
