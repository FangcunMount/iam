package jwks

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeyManagerListKeysPassesLimitAndOffsetInOrder(t *testing.T) {
	repo := &listKeysRepositoryStub{}
	manager := NewKeyManager(repo, nil)

	_, _, err := manager.ListKeys(context.Background(), 0, 10, 3)
	require.NoError(t, err)
	require.Equal(t, 10, repo.lastLimit)
	require.Equal(t, 3, repo.lastOffset)
}

type listKeysRepositoryStub struct {
	lastLimit  int
	lastOffset int
}

func (r *listKeysRepositoryStub) Save(context.Context, *Key) error                { return nil }
func (r *listKeysRepositoryStub) Update(context.Context, *Key) error              { return nil }
func (r *listKeysRepositoryStub) Delete(context.Context, string) error            { return nil }
func (r *listKeysRepositoryStub) FindByKid(context.Context, string) (*Key, error) { return nil, nil }
func (r *listKeysRepositoryStub) FindByStatus(context.Context, KeyStatus) ([]*Key, error) {
	return nil, nil
}
func (r *listKeysRepositoryStub) FindPublishable(context.Context) ([]*Key, error) {
	return nil, nil
}
func (r *listKeysRepositoryStub) FindExpired(context.Context) ([]*Key, error) { return nil, nil }
func (r *listKeysRepositoryStub) FindAll(_ context.Context, limit, offset int) ([]*Key, int64, error) {
	r.lastLimit = limit
	r.lastOffset = offset
	return nil, 0, nil
}
func (r *listKeysRepositoryStub) CountByStatus(context.Context, KeyStatus) (int64, error) {
	return 0, nil
}
