package authentication

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCredentialBuilder_RegisteredBuilders(t *testing.T) {
	// builders for AuthPassword and AuthPhoneOTP are registered via init()
	b, err := getCredentialBuilder(AuthPassword)
	require.NoError(t, err)
	require.NotNil(t, b)

	// password builder should error on missing fields
	_, ierr := b(AuthInput{})
	require.Error(t, ierr)

	// phone otp builder
	b2, err := getCredentialBuilder(AuthPhoneOTP)
	require.NoError(t, err)
	require.NotNil(t, b2)
	_, ierr2 := b2(AuthInput{})
	require.Error(t, ierr2)
}

func TestRegisterCredentialBuilder_Idempotent(t *testing.T) {
	scenario := Scenario("_test_scenario")
	called := false
	RegisterCredentialBuilder(scenario, func(input AuthInput) (AuthCredential, error) {
		called = true
		return nil, nil
	})
	// second registration should be ignored
	RegisterCredentialBuilder(scenario, func(input AuthInput) (AuthCredential, error) {
		return nil, nil
	})

	b, err := getCredentialBuilder(scenario)
	require.NoError(t, err)
	require.NotNil(t, b)
	// calling builder should set called
	_, _ = b(AuthInput{})
	assert.True(t, called)
}

func TestAuthenticater_CreateStrategyMapping(t *testing.T) {
	a := NewAuthenticater(nil, nil, nil, nil, nil, nil)

	// Known scenarios should map to non-nil strategies
	assert.NotNil(t, a.createStrategy(AuthPassword))
	assert.NotNil(t, a.createStrategy(AuthPhoneOTP))
	assert.NotNil(t, a.createStrategy(AuthWxMinip))
	assert.NotNil(t, a.createStrategy(AuthWecom))
	assert.NotNil(t, a.createStrategy(AuthBearerToken))

	// Unknown scenario should return nil
	assert.Nil(t, a.createStrategy(Scenario("unknown")))
}
