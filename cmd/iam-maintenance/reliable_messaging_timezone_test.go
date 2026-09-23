//go:build reliable_messaging

package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Run only with the disposable database provisioned by the proof script.
// Both real connection factories must preserve UTC+8 independently of host TZ.
func TestMaintenanceDatabaseTimezoneMySQL(t *testing.T) {
	require.Equal(t, "1", os.Getenv("IAM_RM_TIMEZONE_REQUIRED"), "isolated proof environment required")
	for name, open := range map[string]func() (*gorm.DB, error){
		"role-database":          func() (*gorm.DB, error) { return roleDatabase("IAM_APISERVER_") },
		"authorization-database": authzConvergeDatabaseFromEnvironment,
	} {
		t.Run(name, func(t *testing.T) {
			db, err := open()
			require.NoError(t, err)
			pool, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = pool.Close() })
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var sessionZone string
			var delta int
			var now, fixedDate time.Time
			require.NoError(t, pool.QueryRowContext(ctx, `SELECT @@session.time_zone,
TIMESTAMPDIFF(SECOND,UTC_TIMESTAMP(),NOW()), NOW(6), CAST('2026-09-22 10:00:00' AS DATETIME(6))`).Scan(&sessionZone, &delta, &now, &fixedDate))
			require.Equal(t, "+08:00", sessionZone)
			require.Equal(t, 8*3600, delta)
			_, offset := fixedDate.Zone()
			require.Equal(t, 8*3600, offset)
			require.Equal(t, "2026-09-22T02:00:00Z", fixedDate.UTC().Format(time.RFC3339))
			require.WithinDuration(t, time.Now(), now, 5*time.Second)
		})
	}
}
