package account

import (
	"context"
	"testing"

	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestMockConsumerCreatorStrategyPrepareDataUsesEmailAsExternalID(t *testing.T) {
	email, err := meta.NewEmail("ref@example.com")
	require.NoError(t, err)

	strategy := NewMockConsumerCreatorStrategy()
	params, err := strategy.PrepareData(context.Background(), CreationInput{
		UserID:      meta.FromUint64(101),
		Email:       email,
		AccountType: TypeMockConsumer,
		Profile:     map[string]string{"nickname": "ref"},
	})
	require.NoError(t, err)
	require.Equal(t, TypeMockConsumer, params.AccountType)
	require.Equal(t, ExternalID("ref@example.com"), params.ExternalID)
	require.Equal(t, MockConsumerAppID, params.AppID)
}

func TestMockConsumerCreatorStrategyRejectsScopedTenantID(t *testing.T) {
	email, err := meta.NewEmail("ref@example.com")
	require.NoError(t, err)

	_, err = NewMockConsumerCreatorStrategy().PrepareData(context.Background(), CreationInput{
		UserID:         meta.FromUint64(101),
		Email:          email,
		AccountType:    TypeMockConsumer,
		ScopedTenantID: meta.FromUint64(1),
	})
	require.Error(t, err)
}
