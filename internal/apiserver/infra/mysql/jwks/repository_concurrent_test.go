package jwks

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	testutil "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/testutil"
	d "github.com/FangcunMount/iam/v2/internal/apiserver/infra/token/keyset"
	testhelpers "github.com/FangcunMount/iam/v2/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
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

func TestKeyRepository_Activate_ConcurrentSingleActive(t *testing.T) {
	db := setupJWKSMySQLConcurrencyDB(t)
	repo := NewKeyRepository(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	old := createTestKey("old-active", d.KeyActive)
	oldBefore := now.Add(-31 * 24 * time.Hour)
	old.NotBefore = &oldBefore
	require.NoError(t, repo.Save(ctx, old))

	dueBefore := now.Add(-30 * 24 * time.Hour)
	graceUntil := now.Add(7 * 24 * time.Hour)
	candidates := []*d.Key{
		createTestKey("candidate-a", d.KeyActive),
		createTestKey("candidate-b", d.KeyActive),
	}
	results := make(chan d.ActivationResult, len(candidates))
	errs := make(chan error, len(candidates))
	var wg sync.WaitGroup
	for _, candidate := range candidates {
		wg.Add(1)
		go func(candidate *d.Key) {
			defer wg.Done()
			result, err := repo.Activate(ctx, d.ActivationRequest{
				Candidate:  candidate,
				Now:        now,
				GraceUntil: graceUntil,
				DueBefore:  &dueBefore,
			})
			results <- result
			errs <- err
		}(candidate)
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	rotated := 0
	for result := range results {
		if result.Activated {
			rotated++
		}
	}
	require.Equal(t, 1, rotated)
	active, err := repo.FindByStatus(ctx, d.KeyActive)
	require.NoError(t, err)
	require.Len(t, active, 1)
	grace, err := repo.FindByStatus(ctx, d.KeyGrace)
	require.NoError(t, err)
	require.Len(t, grace, 1)
	require.Equal(t, "old-active", grace[0].Kid)
}

func setupJWKSMySQLConcurrencyDB(t *testing.T) *gorm.DB {
	t.Helper()
	host := os.Getenv("MYSQL_HOST")
	if host == "" {
		t.Skip("MYSQL_HOST is required for MySQL row-lock concurrency semantics")
	}
	port, err := strconv.Atoi(envOr("MYSQL_PORT", "3306"))
	require.NoError(t, err)
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=UTC",
		envOr("MYSQL_USER", envOr("MYSQL_USERNAME", "iam")),
		os.Getenv("MYSQL_PASSWORD"),
		host,
		port,
		envOr("MYSQL_DATABASE", envOr("MYSQL_DBNAME", "iam_test")),
	)
	db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS `jwks_keys`").Error)
	require.NoError(t, db.Exec(`
CREATE TABLE jwks_keys (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  kid VARCHAR(64) NOT NULL,
  status TINYINT NOT NULL DEFAULT 1,
  kty VARCHAR(32) NOT NULL,
  `+"`use`"+` VARCHAR(16) NOT NULL,
  alg VARCHAR(32) NOT NULL,
  jwk_json JSON NOT NULL,
  not_before DATETIME NULL,
  not_after DATETIME NULL,
  created_at DATETIME NULL,
  updated_at DATETIME NULL,
  deleted_at DATETIME NULL,
  created_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  deleted_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  version INT UNSIGNED NOT NULL DEFAULT 1,
  active_guard TINYINT GENERATED ALWAYS AS (CASE WHEN status = 1 THEN 1 ELSE NULL END) STORED,
  UNIQUE KEY idx_kid (kid),
  UNIQUE KEY uk_jwks_keys_single_active (active_guard),
  KEY idx_status (status)
) ENGINE=InnoDB`).Error)
	t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS `jwks_keys`").Error })
	return db
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
