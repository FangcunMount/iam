package options

import (
	"strings"
	"testing"

	"github.com/FangcunMount/iam/v3/internal/pkg/server"
)

func TestServerRunOptionsCompleteNormalizesMode(t *testing.T) {
	opts := NewServerRunOptions()
	opts.Mode = " Debug "
	if err := opts.Complete(); err != nil {
		t.Fatal(err)
	}
	if opts.Mode != string(server.RuntimeModeDebug) {
		t.Fatalf("Mode = %q, want debug", opts.Mode)
	}
}

func TestServerRunOptionsRejectsRemovedKeysWithoutValues(t *testing.T) {
	sentinel := "removed-config-sentinel"
	timeout := 60
	tests := []struct {
		name string
		key  string
		set  func(*ServerRunOptions)
	}{
		{name: "run mode", key: "server.run-mode", set: func(o *ServerRunOptions) { o.RemovedRunMode = &sentinel }},
		{name: "name", key: "server.name", set: func(o *ServerRunOptions) { o.RemovedName = &sentinel }},
		{name: "read timeout", key: "server.read-timeout", set: func(o *ServerRunOptions) { o.RemovedReadTimeout = &timeout }},
		{name: "write timeout", key: "server.write-timeout", set: func(o *ServerRunOptions) { o.RemovedWriteTimeout = &timeout }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := NewServerRunOptions()
			tt.set(opts)
			errs := opts.Validate()
			if len(errs) != 1 {
				t.Fatalf("Validate() errors = %v, want one", errs)
			}
			message := errs[0].Error()
			if !strings.Contains(message, tt.key) {
				t.Fatalf("error = %q, want key %q", message, tt.key)
			}
			if strings.Contains(message, sentinel) {
				t.Fatalf("error leaked removed value: %q", message)
			}
		})
	}
}

func TestServerRunOptionsApplyToRejectsInvalidMode(t *testing.T) {
	opts := NewServerRunOptions()
	opts.Mode = "production"
	if err := opts.ApplyTo(server.NewConfig()); err == nil {
		t.Fatal("ApplyTo() error = nil, want invalid mode error")
	}
}
