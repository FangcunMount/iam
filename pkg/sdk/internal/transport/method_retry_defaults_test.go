package transport

import (
	"testing"

	"google.golang.org/grpc/codes"
)

func TestDefaultMethodRetryConfigReturnsIndependentCodes(t *testing.T) {
	first := DefaultMethodRetryConfig()
	first.RetryableCodes[0] = codes.Internal

	second := DefaultMethodRetryConfig()
	if second.RetryableCodes[0] == codes.Internal {
		t.Fatal("retryable code mutation leaked into a later default call")
	}
}
