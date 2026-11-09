package process

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"

	componentdb "github.com/FangcunMount/component-base/pkg/database"
	"github.com/FangcunMount/iam/v5/internal/apiserver/config"
	"github.com/FangcunMount/iam/v5/internal/apiserver/options"
	"github.com/FangcunMount/iam/v5/internal/pkg/migration"
	"github.com/stretchr/testify/require"
)

func TestStartupMigrationResumesFreshBootstrapMySQL(t *testing.T) {
	if os.Getenv("IAM_M5_BOOTSTRAP_ATOMIC_FIXTURE") != "1" {
		t.Skip("dedicated disposable MySQL fixture required")
	}
	require.Equal(t, "127.0.0.1", os.Getenv("MYSQL_HOST"))
	require.Equal(t, "iam_test", os.Getenv("MYSQL_DATABASE"))
	host := "127.0.0.1:" + os.Getenv("MYSQL_PORT")
	password := os.Getenv("MYSQL_PASSWORD")
	ctx := t.Context()
	pool := func(t *testing.T, name string) *sql.DB {
		t.Helper()
		connector, err := componentdb.NewMySQLConnector(&componentdb.MySQLConfig{
			Host: host, Username: "root", Password: password, Database: name,
			Location: "Etc/GMT-8", SessionTimeZone: "+08:00",
		})
		require.NoError(t, err)
		db := sql.OpenDB(connector)
		t.Cleanup(func() { _ = db.Close() })
		return db
	}
	admin := pool(t, "iam_test")
	fixture := func(t *testing.T) (*DatabaseManager, *sql.DB, *migration.Migrator) {
		t.Helper()
		name := "iam_m5_entry_" + rand.Text()
		_, err := admin.ExecContext(ctx, "CREATE DATABASE `"+name+"`")
		require.NoError(t, err)
		opts := options.NewOptions()
		opts.MySQLOptions.Host = host
		opts.MySQLOptions.Username = "root"
		opts.MySQLOptions.Password = password
		opts.MySQLOptions.Database = name
		opts.MigrationOptions.Enabled = true
		opts.MigrationOptions.Database = name
		opts.Events.CatalogPath = "../../../configs/events.yaml"
		dm := NewDatabaseManager(&config.Config{Options: opts})
		require.NoError(t, dm.initMySQL())
		require.NoError(t, dm.registry.Init())
		t.Cleanup(func() { _ = dm.Close() })
		db := pool(t, name)
		return dm, db, migration.NewMigrator(db, &migration.Config{Enabled: true, Database: name})
	}
	count := func(t *testing.T, db *sql.DB, query string) int {
		t.Helper()
		var got int
		require.NoError(t, db.QueryRowContext(ctx, query).Scan(&got))
		return got
	}
	assertComplete := func(t *testing.T, db *sql.DB, wantJournal bool) {
		t.Helper()
		require.Equal(t, 45, count(t, db, "SELECT version FROM schema_migrations WHERE dirty=0"))
		require.Equal(t, 2, count(t, db, "SELECT COUNT(DISTINCT event_id) FROM domain_event_outbox WHERE status='pending'"))
		require.Zero(t, count(t, db, "SELECT COUNT(*) FROM rm_outbox"))
		if wantJournal {
			require.Equal(t, 35, count(t, db, "SELECT completed FROM iam_fresh_bootstrap_journal WHERE id=1"))
		} else {
			require.Zero(t, count(t, db, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='iam_fresh_bootstrap_journal'"))
		}
	}
	interrupted := errors.New("fixture interrupted startup")
	for _, phase := range []uint{0, 31, 33, 35} {
		for _, after := range []bool{false, true} {
			if phase == 0 && after {
				continue
			}
			t.Run(fmt.Sprintf("stage_%d_after_hook_%t", phase, after), func(t *testing.T) {
				dm, db, migrator := fixture(t)
				original, err := dm.GetMySQLDB()
				require.NoError(t, err)
				_, _, err = migrator.RunFresh(ctx, func(conn *sql.Conn) (migration.FreshBootstrapHooks, error) {
					if phase == 0 {
						return migration.FreshBootstrapHooks{}, interrupted
					}
					hooks, err := bindFreshBootstrap(ctx, conn, original, dm.config.Events.CatalogPath)
					if err != nil {
						return hooks, err
					}
					at := func(callback func() error) error {
						if !after {
							return interrupted
						}
						if err := callback(); err != nil {
							return err
						}
						return interrupted
					}
					switch phase {
					case 31:
						prepare := hooks.PrepareTenant
						hooks.PrepareTenant = func(record func(*sql.Tx) error) error {
							return at(func() error { return prepare(record) })
						}
					case 33:
						apply := hooks.IndependentRoles
						hooks.IndependentRoles = func() error { return at(apply) }
					case 35:
						apply := hooks.RetireConditions
						hooks.RetireConditions = func() error { return at(apply) }
					}
					return hooks, nil
				})
				require.ErrorIs(t, err, interrupted)
				_, _, err = migrator.RunTo(45)
				require.ErrorContains(t, err, "fresh bootstrap is unfinished")
				if phase == 0 {
					_, err = db.ExecContext(ctx, "UPDATE iam_fresh_bootstrap_journal SET server_uuid='00000000-0000-0000-0000-000000000000'")
					require.NoError(t, err)
					require.ErrorContains(t, dm.runMigrations(), "claim does not match")
					require.Zero(t, count(t, db, "SELECT COUNT(*) FROM schema_migrations"))
					_, err = db.ExecContext(ctx, "UPDATE iam_fresh_bootstrap_journal SET server_uuid=@@server_uuid")
					require.NoError(t, err)
				}
				require.NoError(t, dm.runMigrations())
				assertComplete(t, db, true)
				dm.config.Events.CatalogPath = "/fixture/missing-catalog"
				require.NoError(t, dm.runMigrations())
				assertComplete(t, db, true)
			})
		}
	}
	t.Run("session_lost_after_condition_commit", func(t *testing.T) {
		dm, db, migrator := fixture(t)
		original, err := dm.GetMySQLDB()
		require.NoError(t, err)
		_, _, err = migrator.RunFresh(ctx, func(conn *sql.Conn) (migration.FreshBootstrapHooks, error) {
			hooks, err := bindFreshBootstrap(ctx, conn, original, dm.config.Events.CatalogPath)
			if err != nil {
				return hooks, err
			}
			apply := hooks.RetireConditions
			hooks.RetireConditions = func() error {
				if err := apply(); err != nil {
					return err
				}
				var connectionID uint64
				if err := conn.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&connectionID); err != nil {
					return err
				}
				var selected string
				require.NoError(t, admin.QueryRowContext(ctx, "SELECT DB FROM information_schema.PROCESSLIST WHERE ID=?", connectionID).Scan(&selected))
				require.Equal(t, dm.config.MigrationOptions.Database, selected)
				_, err := admin.ExecContext(ctx, fmt.Sprintf("KILL CONNECTION %d", connectionID))
				require.NoError(t, err)
				return nil
			}
			return hooks, nil
		})
		require.Error(t, err)
		require.Equal(t, 35, count(t, db, "SELECT version FROM schema_migrations WHERE dirty=0"))
		require.Equal(t, 33, count(t, db, "SELECT completed FROM iam_fresh_bootstrap_journal WHERE id=1"))
		require.Equal(t, 2, count(t, db, "SELECT COUNT(*) FROM domain_event_outbox WHERE status='pending'"))
		var before, after string
		query := "SELECT GROUP_CONCAT(CONCAT(event_id,':',SHA2(payload_json,256)) ORDER BY id) FROM domain_event_outbox"
		require.NoError(t, db.QueryRowContext(ctx, query).Scan(&before))
		require.NoError(t, dm.runMigrations())
		assertComplete(t, db, true)
		require.NoError(t, db.QueryRowContext(ctx, query).Scan(&after))
		require.Equal(t, before, after)
	})
	t.Run("existing_unclaimed_upgrade", func(t *testing.T) {
		dm, db, migrator := fixture(t)
		original, err := dm.GetMySQLDB()
		require.NoError(t, err)
		conn, err := db.Conn(ctx)
		require.NoError(t, err)
		defer conn.Close()
		hooks, err := bindFreshBootstrap(ctx, conn, original, dm.config.Events.CatalogPath)
		require.NoError(t, err)
		for _, stage := range []struct {
			version uint
			hook    func() error
		}{
			{31, func() error { return hooks.PrepareTenant(nil) }},
			{33, hooks.IndependentRoles},
			{35, hooks.RetireConditions},
		} {
			version, _, err := migrator.RunTo(stage.version)
			require.NoError(t, err)
			require.Equal(t, stage.version, version)
			require.NoError(t, stage.hook())
		}
		dm.config.Events.CatalogPath = "/fixture/missing-catalog"
		require.NoError(t, dm.runMigrations())
		assertComplete(t, db, false)
	})
}
