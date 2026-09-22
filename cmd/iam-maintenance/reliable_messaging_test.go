package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReliableMessagingPreflightRejectsWritesAndInvalidBounds(t *testing.T) {
	for _, args := range [][]string{
		{"apply"}, {"preflight", "--apply"}, {"preflight", "--target=invalid"},
		{"preflight", "--max-rows=0"}, {"preflight", "--max-rows=1000001"},
		{"preflight", "--timeout=0s"}, {"preflight", "--timeout=6m"}, {"preflight", "extra"},
	} {
		var output bytes.Buffer
		err := run(append([]string{"reliable-messaging"}, args...), &output)
		require.Error(t, err)
		require.Empty(t, output.String())
	}
}
