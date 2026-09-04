package signingkey

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/stretchr/testify/require"
)

func TestKeyManagementAppServiceReadsStringStatusDTOs(t *testing.T) {
	createdAt := time.Date(2026, 4, 28, 9, 30, 0, 0, time.UTC)
	reader := &keyReaderStub{
		active: &ManagedKey{Kid: "active-kid", Algorithm: "RS256", Status: "active", JWK: PublicJWK{Alg: "RS256"}},
		key: &ManagedKey{
			Kid:       "grace-kid",
			Algorithm: "RS384",
			Status:    "grace",
			JWK:       PublicJWK{Alg: "RS384"},
			CreatedAt: createdAt,
			UpdatedAt: createdAt.Add(time.Hour),
		},
		keys: []*ManagedKey{
			{Kid: "active-kid", Algorithm: "RS256", Status: "active", JWK: PublicJWK{Alg: "RS256"}},
			{Kid: "grace-kid", Algorithm: "RS384", Status: "grace", JWK: PublicJWK{Alg: "RS384"}},
		},
		total: 2,
	}
	service := NewKeyManagementAppService(reader, log.New(log.NewOptions()))

	active, err := service.GetActiveKey(context.Background())
	require.NoError(t, err)
	require.Equal(t, "active", active.Status)
	require.Equal(t, "RS256", active.Algorithm)

	key, err := service.GetKeyByKid(context.Background(), "grace-kid")
	require.NoError(t, err)
	require.Equal(t, "grace", key.Status)
	require.Equal(t, "RS384", key.Algorithm)
	require.Equal(t, createdAt, key.CreatedAt)

	list, err := service.ListKeys(context.Background(), ListKeysRequest{Status: "grace", Limit: 20})
	require.NoError(t, err)
	require.Equal(t, "grace", reader.listStatus)
	require.Len(t, list.Keys, 2)
	require.Equal(t, int64(2), list.Total)
}

func TestKeyManagementAppServiceRejectsUnknownStatusBeforePort(t *testing.T) {
	reader := &keyReaderStub{}
	service := NewKeyManagementAppService(reader, log.New(log.NewOptions()))

	_, err := service.ListKeys(context.Background(), ListKeysRequest{Status: "unknown", Limit: 20})
	require.Error(t, err)
	require.Zero(t, reader.listCalls)
}

type keyReaderStub struct {
	active     *ManagedKey
	key        *ManagedKey
	keys       []*ManagedKey
	total      int64
	listStatus string
	listCalls  int
}

func (s *keyReaderStub) GetActiveKey(context.Context) (*ManagedKey, error) {
	return s.active, nil
}

func (s *keyReaderStub) GetKeyByKid(context.Context, string) (*ManagedKey, error) {
	return s.key, nil
}

func (s *keyReaderStub) ListKeys(_ context.Context, status string, _, _ int) ([]*ManagedKey, int64, error) {
	s.listCalls++
	s.listStatus = status
	return s.keys, s.total, nil
}
