package process

import (
	"testing"
	"time"

	"github.com/FangcunMount/component-base/pkg/messaging"
)

func TestNormalizeNSQConfigAppliesRuntimeDefaults(t *testing.T) {
	cfg := &messaging.Config{}

	got := normalizeNSQConfig(cfg)

	if got != cfg {
		t.Fatal("normalizeNSQConfig returned a different config pointer")
	}
	if got.NSQ.MsgTimeout != 60*time.Second {
		t.Fatalf("MsgTimeout = %v, want 60s", got.NSQ.MsgTimeout)
	}
	if got.NSQ.RequeueDelay != 5*time.Second {
		t.Fatalf("RequeueDelay = %v, want 5s", got.NSQ.RequeueDelay)
	}
	if len(got.NSQ.LookupdAddrs) != 1 || got.NSQ.LookupdAddrs[0] != "127.0.0.1:4161" {
		t.Fatalf("LookupdAddrs = %#v, want default lookupd", got.NSQ.LookupdAddrs)
	}
	if got.NSQ.NSQdAddr != "127.0.0.1:4150" {
		t.Fatalf("NSQdAddr = %q, want default nsqd", got.NSQ.NSQdAddr)
	}
	if got.NSQ.MaxAttempts != 5 {
		t.Fatalf("MaxAttempts = %d, want 5", got.NSQ.MaxAttempts)
	}
	if got.NSQ.MaxInFlight != 200 {
		t.Fatalf("MaxInFlight = %d, want 200", got.NSQ.MaxInFlight)
	}
}

func TestNormalizeNSQConfigPreservesConfiguredValues(t *testing.T) {
	cfg := &messaging.Config{NSQ: messaging.NSQConfig{
		MsgTimeout:   11 * time.Second,
		RequeueDelay: 12 * time.Second,
		LookupdAddrs: []string{"lookupd:4161"},
		NSQdAddr:     "nsqd:4150",
		MaxAttempts:  9,
		MaxInFlight:  33,
	}}

	got := normalizeNSQConfig(cfg)

	if got.NSQ.MsgTimeout != 11*time.Second ||
		got.NSQ.RequeueDelay != 12*time.Second ||
		len(got.NSQ.LookupdAddrs) != 1 || got.NSQ.LookupdAddrs[0] != "lookupd:4161" ||
		got.NSQ.NSQdAddr != "nsqd:4150" ||
		got.NSQ.MaxAttempts != 9 ||
		got.NSQ.MaxInFlight != 33 {
		t.Fatalf("configured NSQ values were not preserved: %#v", got.NSQ)
	}
}
