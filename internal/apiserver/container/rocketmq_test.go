package container

import (
	"github.com/FangcunMount/iam/v5/internal/apiserver/options"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRetiredPolicyProviderRejectedBeforeDatabaseAndClient(t *testing.T) {
	for _, provider := range []string{"", "nsq"} {
		t.Run(provider, func(t *testing.T) {
			c := &Container{runtimeOptions: RuntimeOptions{Messaging: options.MessagingOptions{Provider: provider}}}
			require.ErrorContains(t, c.initRocketMQPolicySync(nil), "NSQ is retired")
			require.Nil(t, c.sdkPolicySync)
		})
	}
}
