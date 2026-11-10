package migration

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratedb "github.com/golang-migrate/migrate/v4/database"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

const freshBootstrapTable = "iam_fresh_bootstrap_journal"

const (
	legacyBootstrapDelivery      = "legacy"
	transactionBootstrapDelivery = "transaction-v1"
)

// FreshBootstrapHooks are bound to the same SQL session as the migration
// driver. PrepareTenant must record its checkpoint in its business transaction.
// The other two hooks must be safe to retry from their existing receipts.
type FreshBootstrapHooks struct {
	PrepareTenant      func(func(*sql.Tx) error) error
	IndependentRoles   func() error
	RetireConditions   func() error
	VerifyNotification func(uint) error
}

type migrationSession struct {
	conn     *sql.Conn
	instance *migrate.Migrate
	database string
	lock     string
}

func (m *Migrator) lockSession(ctx context.Context) (*migrationSession, error) {
	conn, err := m.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	var name string
	if err = conn.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&name); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if name == "" || m.config.Database != "" && name != m.config.Database {
		_ = conn.Close()
		return nil, errors.New("selected migration database differs from configuration")
	}
	lock, err := migratedb.GenerateAdvisoryLockId(name + ":schema_migrations")
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	var owned bool
	if err = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,10)", lock).Scan(&owned); err != nil || !owned {
		// An interrupted lock acquisition must not return a possibly locked
		// connection to the pool.
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		_ = conn.Close()
		if err != nil {
			return nil, err
		}
		return nil, migratedb.ErrLocked
	}
	return &migrationSession{conn: conn, database: name, lock: lock}, nil
}

func (s *migrationSession) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var released bool
	var held bool
	if err := s.conn.QueryRowContext(ctx, "SELECT RELEASE_LOCK(?)", s.lock).Scan(&released); err != nil || !released ||
		s.conn.QueryRowContext(ctx, "SELECT COALESCE(IS_USED_LOCK(?)=CONNECTION_ID(),0)", s.lock).Scan(&held) != nil || held {
		_ = s.conn.Raw(func(any) error { return driver.ErrBadConn })
	}
	if s.instance != nil {
		_, _ = s.instance.Close()
	} else {
		_ = s.conn.Close()
	}
}

func (s *migrationSession) open(ctx context.Context) (*migrate.Migrate, error) {
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		return nil, err
	}
	database, err := migratemysql.WithConnection(ctx, s.conn, &migratemysql.Config{
		DatabaseName: s.database, MigrationsTable: "schema_migrations",
	})
	if err != nil {
		_ = source.Close()
		return nil, err
	}
	s.instance, err = migrate.NewWithInstance("iofs", source, "mysql", database)
	if err != nil {
		_ = source.Close()
	}
	return s.instance, err
}

type bootstrapJournal struct {
	database, server, delivery string
	completed                  uint
}

func (s *migrationSession) journal(ctx context.Context) (*bootstrapJournal, error) {
	var engine string
	err := s.conn.QueryRowContext(ctx, `SELECT COALESCE(ENGINE,'') FROM information_schema.TABLES
		WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?`, freshBootstrapTable).Scan(&engine)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if engine != "InnoDB" {
		return nil, errors.New("fresh bootstrap journal requires InnoDB")
	}
	rows, err := s.conn.QueryContext(ctx, "SELECT id,db_name,server_uuid,delivery,completed FROM iam_fresh_bootstrap_journal LIMIT 2")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var id int
	var state bootstrapJournal
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("fresh bootstrap journal has no claim")
	}
	if err := rows.Scan(&id, &state.database, &state.server, &state.delivery, &state.completed); err != nil {
		return nil, err
	}
	if id != 1 || state.database == "" || state.server == "" ||
		(state.delivery != legacyBootstrapDelivery && state.delivery != transactionBootstrapDelivery) || rows.Next() {
		return nil, errors.New("invalid fresh bootstrap journal")
	}
	switch state.completed {
	case 0, 31, 33, 35:
		return &state, rows.Err()
	default:
		return nil, errors.New("invalid fresh bootstrap checkpoint")
	}
}

func (s *migrationSession) openWithoutPendingBootstrap(ctx context.Context) (*migrate.Migrate, error) {
	state, err := s.journal(ctx)
	if err != nil {
		return nil, err
	}
	if state != nil && state.completed != 35 {
		return nil, errors.New("fresh bootstrap is unfinished; resume it through the startup migration entry")
	}
	instance, err := s.open(ctx)
	if err != nil || state == nil {
		return instance, err
	}
	version, _, err := instance.Version()
	if err != nil || version < state.completed {
		return nil, errors.New("migration ledger predates its fresh bootstrap checkpoint")
	}
	return instance, nil
}

// RunFresh runs the existing startup migration path. An existing unclaimed
// installation follows the normal upgrade path without bootstrap hooks.
func (m *Migrator) RunFresh(ctx context.Context, bind func(*sql.Conn) (FreshBootstrapHooks, error)) (uint, bool, error) {
	return m.runFresh(ctx, legacyBootstrapDelivery, "", bind)
}

// RunFreshForTransactionDelivery claims only a fresh database. A previous
// claim cannot switch delivery paths, even after a hook has committed.
func (m *Migrator) RunFreshForTransactionDelivery(ctx context.Context, serverUUID string, bind func(*sql.Conn) (FreshBootstrapHooks, error)) (uint, bool, error) {
	if m.config.Database == "" || serverUUID == "" || bind == nil {
		return 0, false, errors.New("transaction bootstrap requires a pinned database, server and hooks")
	}
	return m.runFresh(ctx, transactionBootstrapDelivery, serverUUID, bind)
}

func (m *Migrator) runFresh(ctx context.Context, delivery, expectedServer string, bind func(*sql.Conn) (FreshBootstrapHooks, error)) (uint, bool, error) {
	if !m.config.Enabled {
		return 0, false, nil
	}
	s, err := m.lockSession(ctx)
	if err != nil {
		return 0, false, err
	}
	defer s.close()
	state, err := s.journal(ctx)
	if err != nil {
		return 0, false, err
	}
	var server string
	if expectedServer != "" {
		if err := s.conn.QueryRowContext(ctx, "SELECT @@server_uuid").Scan(&server); err != nil {
			return 0, false, err
		}
		if server != expectedServer {
			return 0, false, errors.New("selected MySQL server differs from the reviewed UUID")
		}
		if state == nil {
			if err := s.requireEmptyTransactionBootstrap(ctx); err != nil {
				return 0, false, err
			}
			state, err = s.claim(ctx, server, delivery)
			if err != nil {
				return 0, false, err
			}
		}
		if state == nil || state.database != s.database || state.server != server || state.delivery != delivery {
			return 0, false, errors.New("fresh bootstrap claim does not match the original database and delivery")
		}
	}
	instance, err := s.open(ctx)
	if err != nil {
		return 0, false, err
	}
	before, dirty, err := instance.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		before = 0
	} else if err != nil {
		return 0, false, err
	}
	if dirty || state != nil && before < state.completed {
		return before, false, errors.New("migration ledger conflicts with fresh bootstrap")
	}
	if state == nil && before != 0 {
		return finishUp(instance, before, false)
	}
	if state != nil && state.delivery != delivery {
		return before, false, errors.New("fresh bootstrap notification delivery does not match the database claim")
	}
	if state != nil && state.completed == 35 && delivery == legacyBootstrapDelivery {
		return finishUp(instance, before, false)
	}
	if bind == nil {
		return before, false, errors.New("fresh bootstrap hooks are required")
	}
	if server == "" {
		if err := s.conn.QueryRowContext(ctx, "SELECT @@server_uuid").Scan(&server); err != nil {
			return before, false, err
		}
	}
	if state == nil {
		var tables int
		if err := s.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME <> 'schema_migrations'").Scan(&tables); err != nil {
			return before, false, err
		}
		if tables != 0 {
			return before, false, errors.New("fresh bootstrap requires an empty database")
		}
		state, err = s.claim(ctx, server, delivery)
		if err != nil {
			return before, false, err
		}
	}
	if state == nil || state.database != s.database || state.server != server || state.delivery != delivery {
		return before, false, errors.New("fresh bootstrap claim does not match the original database")
	}
	hooks, err := bind(s.conn)
	if err != nil {
		return before, false, err
	}
	if hooks.PrepareTenant == nil || hooks.IndependentRoles == nil || hooks.RetireConditions == nil {
		return before, false, errors.New("incomplete fresh bootstrap hooks")
	}
	if delivery == transactionBootstrapDelivery {
		if hooks.VerifyNotification == nil {
			return before, false, errors.New("transaction bootstrap notification verification is required")
		}
		for _, phase := range []uint{33, 35} {
			if phase <= state.completed {
				if err := hooks.VerifyNotification(phase); err != nil {
					return before, false, err
				}
			}
		}
	}
	current, completed, changed := before, state.completed, false
	for _, phase := range []uint{31, 33, 35} {
		if phase <= completed {
			continue
		}
		if current > phase {
			return current, changed, errors.New("migration ledger passed an unfinished fresh bootstrap stage")
		}
		if current < phase {
			if err := instance.Migrate(phase); err != nil {
				return current, changed, err
			}
			current, changed = phase, true
		}
		record := func(conn interface {
			ExecContext(context.Context, string, ...any) (sql.Result, error)
		}) error {
			result, err := conn.ExecContext(ctx, `UPDATE iam_fresh_bootstrap_journal SET completed=?
				WHERE id=1 AND db_name=? AND server_uuid=? AND delivery=? AND completed=?`, phase, s.database, server, delivery, completed)
			if err != nil {
				return err
			}
			if count, err := result.RowsAffected(); err != nil || count != 1 {
				return errors.New("fresh bootstrap checkpoint changed")
			}
			return nil
		}
		switch phase {
		case 31:
			err = hooks.PrepareTenant(func(tx *sql.Tx) error {
				var owned bool
				if err := tx.QueryRowContext(ctx, "SELECT COALESCE(IS_USED_LOCK(?)=CONNECTION_ID(),0)", s.lock).Scan(&owned); err != nil {
					return err
				}
				if !owned {
					return errors.New("fresh bootstrap checkpoint is outside the migration session")
				}
				return record(tx)
			})
		case 33:
			err = hooks.IndependentRoles()
		case 35:
			err = hooks.RetireConditions()
		}
		if err == nil && phase != 31 && delivery == transactionBootstrapDelivery {
			err = hooks.VerifyNotification(phase)
		}
		if err == nil && phase != 31 {
			err = record(s.conn)
		}
		if err != nil {
			return current, changed, fmt.Errorf("fresh bootstrap at %d: %w", phase, err)
		}
		saved, err := s.journal(ctx)
		if err != nil {
			return current, true, err
		}
		if saved == nil || saved.completed != phase || saved.database != s.database || saved.server != server || saved.delivery != delivery {
			return current, true, errors.New("fresh bootstrap checkpoint was not persisted")
		}
		completed, changed = phase, true
	}
	return finishUp(instance, before, changed)
}

func (s *migrationSession) requireEmptyTransactionBootstrap(ctx context.Context) error {
	// Check the direct grants used by the immutable migration chain and
	// the visibility required to prove the selected database is empty.
	var granted int
	err := s.conn.QueryRowContext(ctx, `WITH principal AS (
		SELECT CONCAT(QUOTE(LEFT(CURRENT_USER(),CHAR_LENGTH(CURRENT_USER())-CHAR_LENGTH(SUBSTRING_INDEX(CURRENT_USER(),'@',-1))-1)),
			'@',QUOTE(SUBSTRING_INDEX(CURRENT_USER(),'@',-1))) AS grantee
	), visible_grants AS (
		SELECT PRIVILEGE_TYPE FROM information_schema.SCHEMA_PRIVILEGES JOIN principal USING (GRANTEE) WHERE TABLE_SCHEMA=DATABASE()
		UNION
		SELECT PRIVILEGE_TYPE FROM information_schema.USER_PRIVILEGES JOIN principal USING (GRANTEE)
	) SELECT COUNT(*) FROM visible_grants WHERE PRIVILEGE_TYPE IN (
		'SELECT','INSERT','UPDATE','DELETE','CREATE','ALTER','DROP','INDEX','REFERENCES',
		'TRIGGER','EXECUTE','EVENT','CREATE TEMPORARY TABLES','SHOW VIEW')`).Scan(&granted)
	if err != nil {
		return err
	}
	if granted != 14 {
		return errors.New("fresh bootstrap requires direct migration and metadata grants on the selected database")
	}
	var occupied bool
	err = s.conn.QueryRowContext(ctx, `SELECT
		EXISTS(SELECT 1 FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()) OR
		EXISTS(SELECT 1 FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA=DATABASE()) OR
		EXISTS(SELECT 1 FROM information_schema.EVENTS WHERE EVENT_SCHEMA=DATABASE())`).Scan(&occupied)
	if err != nil {
		return err
	}
	if occupied {
		return errors.New("transaction delivery bootstrap requires a fresh database claim")
	}
	return nil
}

func (s *migrationSession) claim(ctx context.Context, server, delivery string) (*bootstrapJournal, error) {
	// InnoDB CREATE TABLE ... SELECT installs the initial claim in the
	// same statement. A missing or malformed claim is never recreated.
	if _, err := s.conn.ExecContext(ctx, `CREATE TABLE iam_fresh_bootstrap_journal (
		id TINYINT UNSIGNED PRIMARY KEY CHECK (id=1), db_name VARBINARY(64) NOT NULL,
		server_uuid CHAR(36) CHARACTER SET ascii NOT NULL,
		delivery VARBINARY(16) NOT NULL CHECK (delivery IN ('legacy','transaction-v1')),
		completed TINYINT UNSIGNED NOT NULL CHECK (completed IN (0,31,33,35))
	) ENGINE=InnoDB AS SELECT 1 AS id, ? AS db_name, ? AS server_uuid, ? AS delivery, 0 AS completed`, s.database, server, delivery); err != nil {
		return nil, err
	}
	return s.journal(ctx)
}

func finishUp(instance *migrate.Migrate, before uint, changed bool) (uint, bool, error) {
	if err := instance.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return before, changed, fmt.Errorf("migration failed: %w", err)
	}
	head, dirty, err := instance.Version()
	if err != nil {
		return before, changed, fmt.Errorf("failed to get new version: %w", err)
	}
	if dirty {
		err = errors.New("migration finished with dirty schema")
	}
	return head, changed || head != before, err
}
