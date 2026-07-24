package options

import (
	"strings"
	"testing"
)

func TestMySQLOptionsLogLevelValidation(t *testing.T) {
	t.Parallel()

	for _, level := range []int{1, 2, 3, 4} {
		opts := NewMySQLOptions()
		opts.LogLevel = level
		if errs := opts.Validate(); len(errs) != 0 {
			t.Fatalf("level %d rejected: %v", level, errs)
		}
	}
	for _, level := range []int{-1, 0, 5} {
		opts := NewMySQLOptions()
		opts.LogLevel = level
		errs := opts.Validate()
		if len(errs) != 1 || !strings.Contains(errs[0].Error(), "mysql.log-level") {
			t.Fatalf("level %d errors = %v, want key-only range error", level, errs)
		}
		if strings.Contains(errs[0].Error(), "-1") || strings.Contains(errs[0].Error(), " 0") || strings.Contains(errs[0].Error(), " 5") {
			t.Fatalf("level %d leaked configured value: %v", level, errs[0])
		}
	}
}
