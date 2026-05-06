package testutil

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// firstNonEmpty returns the first non-empty string from the args
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// OpenDBForIntegrationTest returns a *gorm.DB for integration/concurrent tests.
// It prefers MySQL when relevant env vars are set (supports multiple common names),
// otherwise falls back to an in-memory SQLite shared DB. The helper also performs
// AutoMigrate on provided models.
func OpenDBForIntegrationTest(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()

	host := firstNonEmpty(os.Getenv("MYSQL_HOST"), os.Getenv("IAM_APISERVER_MYSQL_HOST"), os.Getenv("MYSQL_HOSTNAME"))
	if host != "" {
		port := firstNonEmpty(os.Getenv("MYSQL_PORT"), "3306")
		user := firstNonEmpty(os.Getenv("MYSQL_USER"), os.Getenv("MYSQL_USERNAME"), os.Getenv("IAM_APISERVER_MYSQL_USERNAME"), os.Getenv("IAM_APISERVER_MYSQL_USER"))
		pass := firstNonEmpty(os.Getenv("MYSQL_PASSWORD"), os.Getenv("MYSQL_PASS"), os.Getenv("IAM_APISERVER_MYSQL_PASSWORD"))
		dbName := firstNonEmpty(os.Getenv("MYSQL_DATABASE"), os.Getenv("MYSQL_DBNAME"), os.Getenv("IAM_APISERVER_MYSQL_DATABASE"), os.Getenv("IAM_APISERVER_MYSQL_DBNAME"))
		if dbName == "" {
			dbName = "infrastructure_dev"
		}

		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&loc=Local", user, pass, host, port, dbName)
		db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
		require.NoError(t, err, "failed to open MySQL connection for integration test")

		if len(models) > 0 {
			require.NoError(t, db.AutoMigrate(models...), "failed to auto-migrate models on MySQL")
		}

		return db
	}

	// Fallback to sqlite using a per-test temporary file-backed DB to avoid
	// cross-test locking when tests run concurrently. We enable WAL and a busy
	// timeout via the DSN so the sqlite driver applies them at open time. This
	// is more reliable across driver versions than only issuing PRAGMA after open.
	tmpFile, err := os.CreateTemp("", "iam_test_db-*.db")
	require.NoError(t, err, "failed to create temp sqlite file for test")
	// ensure the file is removed when the test finishes
	t.Cleanup(func() {
		_ = os.Remove(tmpFile.Name())
	})
	// close the os.File handle returned by CreateTemp; GORM/sqlite will open by path
	_ = tmpFile.Close()

	// Use a file: DSN so that journal_mode and busy_timeout can be applied
	// as DSN query params and take effect immediately.
	// increase busy timeout to give retries more time under high contention
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=10000", tmpFile.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err, "failed to open sqlite DB file for test")

	// also try to set pragmas as a best-effort fallback; ignore errors
	sqlDB, derr := db.DB()
	if derr == nil {
		_, _ = sqlDB.ExecContext(t.Context(), "PRAGMA journal_mode=WAL;")
		_, _ = sqlDB.ExecContext(t.Context(), "PRAGMA busy_timeout=10000;")
		// Limit the underlying connection pool to a single connection to
		// serialize DB access at the connection level for each test DB. This
		// reduces sqlite "database table is locked" errors that occur when
		// many goroutines open parallel connections to the same file.
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
	}

	if len(models) > 0 {
		require.NoError(t, db.AutoMigrate(models...), "failed to auto-migrate models on sqlite")
	}

	return db
}

// RetryOnDBLocked runs the provided operation and, if it fails with a
// sqlite "database is locked" (or similar transient) error, retries it with
// exponential backoff and jitter. This utility is intended for tests only to
// reduce noise from transient sqlite locking under heavy concurrency.
func RetryOnDBLocked(op func() error) error {
	const maxAttempts = 8
	baseDelay := 10 * time.Millisecond

	var lastErr error
	// Seed rand for jitter; fine even if reseeded repeatedly in tests
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := op(); err != nil {
			lastErr = err
			// detect common sqlite transient lock/busy messages
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "database is locked") || strings.Contains(msg, "database is busy") || strings.Contains(msg, "database table is locked") {
				// exponential backoff with jitter
				sleep := baseDelay * (1 << attempt)
				// cap sleep to a reasonable bound
				if sleep > 500*time.Millisecond {
					sleep = 500 * time.Millisecond
				}
				// add jitter up to sleep
				jitter := time.Duration(r.Int63n(int64(sleep)))
				time.Sleep(sleep + jitter)
				continue
			}
			// non-transient error: return immediately
			return err
		}
		return nil
	}
	return lastErr
}
