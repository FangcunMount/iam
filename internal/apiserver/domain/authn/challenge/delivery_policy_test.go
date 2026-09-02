package challenge

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResolveSMSOTPDeliveryPolicyUsesDefaults(t *testing.T) {
	t.Parallel()
	policy := ResolveSMSOTPDeliveryPolicy(SMSOTPDeliveryConfig{})
	require.Equal(t, DefaultSMSOTPTTL, policy.TTL)
	require.Equal(t, DefaultSMSOTPSendCooldown, policy.Cooldown)
	require.Equal(t, DefaultSMSOTPCodeLen, policy.CodeLen)
	require.Equal(t, DefaultSMSOTPHourlyLimit, policy.EffectiveHourlyLimit())
	require.Equal(t, DefaultSMSOTPDailyLimit, policy.EffectiveDailyLimit())
}

func TestResolveSMSOTPDeliveryPolicyHonorsOverrides(t *testing.T) {
	t.Parallel()
	policy := ResolveSMSOTPDeliveryPolicy(SMSOTPDeliveryConfig{
		TTL:         3 * time.Minute,
		Cooldown:    30 * time.Second,
		CodeLen:     8,
		HourlyLimit: 7,
		DailyLimit:  11,
	})
	require.Equal(t, 3*time.Minute, policy.TTL)
	require.Equal(t, 30*time.Second, policy.Cooldown)
	require.Equal(t, 8, policy.CodeLen)
	require.Equal(t, 7, policy.EffectiveHourlyLimit())
	require.Equal(t, 11, policy.EffectiveDailyLimit())
}

func TestResolveSMSOTPDeliveryPolicyCapsCodeLen(t *testing.T) {
	t.Parallel()
	policy := ResolveSMSOTPDeliveryPolicy(SMSOTPDeliveryConfig{CodeLen: 99})
	require.Equal(t, MaxSMSOTPCodeLen, policy.CodeLen)
}

func TestResolveSMSOTPDeliveryPolicyDisablesQuotaWhenNegative(t *testing.T) {
	t.Parallel()
	policy := ResolveSMSOTPDeliveryPolicy(SMSOTPDeliveryConfig{
		HourlyLimit: -1,
		DailyLimit:  -1,
	})
	require.Equal(t, 0, policy.EffectiveHourlyLimit())
	require.Equal(t, 0, policy.EffectiveDailyLimit())
}

func TestQuotaDimensionsOrder(t *testing.T) {
	t.Parallel()
	dims := QuotaDimensions()
	require.Len(t, dims, 2)
	require.Equal(t, QuotaDimensionHourly, dims[0].Dimension)
	require.Equal(t, QuotaWindowHourly, dims[0].Window)
	require.Equal(t, QuotaDimensionDaily, dims[1].Dimension)
	require.Equal(t, QuotaWindowDaily, dims[1].Window)
}
