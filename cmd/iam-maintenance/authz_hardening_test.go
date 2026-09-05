package main

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHardeningRequiresReviewFingerprintAndExplicitOperation(t *testing.T) {
	for _, args := range [][]string{{"authz-hardening"}, {"authz-hardening", "unknown"}, {"authz-hardening", "apply"}, {"authz-hardening", "apply", "--confirm=APPLY_AUTHZ_HARDENING"}, {"authz-hardening", "preflight", "--confirm=APPLY_AUTHZ_HARDENING"}, {"authz-hardening", "evidence"}, {"authz-hardening", "preflight", "--timeout=0s"}} {
		var output bytes.Buffer
		require.Error(t, run(args, &output))
		require.Empty(t, output.String())
	}
}
