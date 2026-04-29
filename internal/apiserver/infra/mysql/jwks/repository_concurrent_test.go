package jwks

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	testutil "github.com/FangcunMount/iam/internal/apiserver/application/uc/testutil"
	d "github.com/FangcunMount/iam/internal/apiserver/infra/token/keyset"
	testhelpers "github.com/FangcunMount/iam/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

// 并发保存相同 kid 的 Key，期望只有 1 条记录被写入，其他请求返回业务错误 ErrKeyAlreadyExists
func TestKeyRepository_Save_ConcurrentDuplicateDetection(t *testing.T) {
	db := testutil.SetupTestDB(t)
	require.NoError(t, db.AutoMigrate(&KeyPO{}))

	repo := NewKeyRepository(db)
	ctx := context.Background()

	const concurrency = 100
	var wg sync.WaitGroup
	wg.Add(concurrency)

	errs := make(chan error, concurrency)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < concurrency; i++ {
		delay := rng.Intn(8)
		go func(delayInt int) {
			defer wg.Done()
			// small jitter to reduce table-lock collisions
			time.Sleep(time.Millisecond * time.Duration(delayInt))
			n := "n"
			e := "AQAB"
			key := d.NewKey("dup-kid", d.PublicJWK{Kty: "RSA", Use: "sig", Alg: "RS256", Kid: "dup-kid", N: &n, E: &e}, d.WithStatus(d.KeyActive))
			if err := testhelpers.RetryOnDBLocked(func() error { return repo.Save(ctx, key) }); err != nil {
				errs <- err
				return
			}
			errs <- nil
		}(delay)
	}

	wg.Wait()
	close(errs)

	var success int
	var mappedCount int
	for e := range errs {
		if e == nil {
			success++
			continue
		}
		if perrors.IsCode(e, code.ErrKeyAlreadyExists) {
			mappedCount++
		}
	}

	require.Equal(t, 1, success, "only one save should succeed")
	require.GreaterOrEqual(t, mappedCount, 1, "at least one error should be mapped to ErrKeyAlreadyExists")

	var cnt int64
	require.NoError(t, db.Model(&KeyPO{}).Where("kid = ?", "dup-kid").Count(&cnt).Error)
	require.Equal(t, int64(1), cnt)
}
