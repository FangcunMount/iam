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

func TestRetiredNSQConsumerCannotBeEnabledByPreparation(t *testing.T) {
	opts := NewOptions()
	opts.NSQOptions.ConsumerSDKEnabled = true
	require.ErrorContains(t, errors.Join(opts.Validate()...), "NSQ publisher and policy consumer are retired")
	opts.NSQOptions.Enabled = true
	opts.NSQOptions.NSQdHTTPAddrs = []string{"http://127.0.0.1:4151"}
	require.ErrorContains(t, errors.Join(opts.Validate()...), "NSQ publisher and policy consumer are retired")
	for _, provider := range []string{"", "nsq", "unknown"} {
		require.Error(t, (MessagingOptions{Provider: provider}).Validate())
	}
}
