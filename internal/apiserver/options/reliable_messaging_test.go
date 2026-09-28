package options

import (
	"errors"
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

func TestSDKPolicyConsumerRequiresExplicitNSQPreparation(t *testing.T) {
	opts := NewOptions()
	opts.NSQOptions.ConsumerSDKEnabled = true
	require.ErrorContains(t, errors.Join(opts.Validate()...), "requires nsq.enabled")
	opts.NSQOptions.Enabled = true
	require.ErrorContains(t, errors.Join(opts.Validate()...), "requires explicit nsq.nsqd-http-addrs")
	opts.NSQOptions.NSQdHTTPAddrs = []string{"http://127.0.0.1:4151"}
	for _, err := range opts.Validate() {
		require.NotContains(t, err.Error(), "nsq.consumer-sdk-enabled")
	}
}
