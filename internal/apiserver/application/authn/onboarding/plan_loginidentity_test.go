package onboarding

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOnboardingPlanCreatesLongTermCredentialOnlyForPasswordScenarios(t *testing.T) {
	tests := []struct {
		name           string
		req            OnboardingRequest
		needCredential bool
	}{
		{
			name: "opera password",
			req: OnboardingRequest{
				Scenario: OnboardOperaPassword,
			},
			needCredential: true,
		},
		{
			name: "mock consumer password",
			req: OnboardingRequest{
				Scenario: OnboardMockConsumerPassword,
			},
			needCredential: true,
		},
		{
			name: "phone otp",
			req: OnboardingRequest{
				Scenario: OnboardPhoneOTP,
			},
			needCredential: false,
		},
		{
			name: "wechat mini",
			req: OnboardingRequest{
				Scenario: OnboardWechatMini,
			},
			needCredential: false,
		},
		{
			name: "wecom",
			req: OnboardingRequest{
				Scenario: OnboardWecom,
			},
			needCredential: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if tt.req.Scenario == OnboardOperaPassword {
				tt.req.ScopedTenantID = 1
			}
			plan, err := BuildPlan(tt.req)
			require.NoError(t, err)
			require.Equal(t, tt.needCredential, plan.NeedCredential)
		})
	}
}
