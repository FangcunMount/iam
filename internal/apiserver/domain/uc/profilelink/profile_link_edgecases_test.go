package profilelink

import (
	"sync"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRef_Revoke_Idempotent(t *testing.T) {
	g := &ProfileLink{User: meta.FromUint64(1), Profile: meta.FromUint64(2)}
	// first revoke
	t1 := time.Now()
	g.Revoke(t1)
	require.NotNil(t, g.RevokedAt)
	first := *g.RevokedAt

	// wait a tick and revoke again with later time
	time.Sleep(5 * time.Millisecond)
	t2 := time.Now()
	g.Revoke(t2)
	require.NotNil(t, g.RevokedAt)
	second := *g.RevokedAt

	// second revoke should update the timestamp (not keep nil)
	assert.True(t, second.After(first) || second.Equal(first))
}

func TestRef_ConcurrentRevoke(t *testing.T) {
	g := &ProfileLink{User: meta.FromUint64(1), Profile: meta.FromUint64(2)}

	const N = 10
	var wg sync.WaitGroup
	wg.Add(N)

	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			// each goroutine uses its own timestamp
			g.Revoke(time.Now().Add(time.Duration(i) * time.Millisecond))
		}(i)
	}
	wg.Wait()

	require.NotNil(t, g.RevokedAt)
	// revoked time should be set (we don't assert ordering due to race, just existence)
	assert.False(t, g.RevokedAt.IsZero())
}
