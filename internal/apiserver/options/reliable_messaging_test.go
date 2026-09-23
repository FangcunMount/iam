package options

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReliableMessagingDefaultsAndBounds(t *testing.T) {
	opts := NewEventOptions().ReliableMessaging
	require.False(t, opts.Enabled)
	opts.Enabled = true
	require.NoError(t, opts.Validate())
	opts.ShutdownTimeout = opts.PublishTimeout
	require.Error(t, opts.Validate())
	opts = DefaultReliableMessagingOptions()
	opts.Enabled = true
	opts.WriteTimeout = opts.Lease
	require.Error(t, opts.Validate())
	opts = DefaultReliableMessagingOptions()
	opts.Enabled = true
	opts.RestartDelay = 2 * time.Minute
	require.Error(t, opts.Validate())
	opts.Enabled = false
	require.NoError(t, opts.Validate(), "disabled compatibility path ignores unused settings")
}
