//go:build integration

package main

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	keyspace "github.com/FangcunMount/component-base/pkg/redis/keyspace"
	sessiondomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	sessioncache "github.com/FangcunMount/iam/v5/internal/apiserver/infra/cache/redis"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/conditionretire"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/rolemodel"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/FangcunMount/iam/v5/internal/pkg/migration"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	driver "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// The runner owns a fresh iam_mock_* schema migrated through the original full
// 1..40 resource chain and a dedicated Redis logical DB. Only an empty schema is
// initialized; these tests never force dirty state or flush an existing Redis DB.
func TestIAMMockCohortNative(t *testing.T) {
	if os.Getenv("IAM_MOCK_COHORT_NATIVE_REQUIRED") != "1" {
		t.Skip("explicit isolated MySQL+Redis native lane required")
	}
	dsn, err := driver.ParseDSN(os.Getenv("IAM_MOCK_COHORT_NATIVE_MYSQL_DSN"))
	if err != nil || !strings.HasPrefix(dsn.DBName, "iam_mock_") || dsn.Net != "tcp" {
		t.Fatal("dedicated native schema required")
	}
	host, _, err := net.SplitHostPort(dsn.Addr)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		t.Fatal("native MySQL must use explicit loopback")
	}
	dsn.ParseTime = true
	dsn.Loc = time.UTC
	dsn.MultiStatements = false
	catalog, err := os.ReadFile(filepath.Join("..", "..", "..", "configs", "events.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	prepareNativeSchema(t, dsn, catalog)
	db, err := sql.Open("mysql", dsn.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	opt, err := redis.ParseURL(os.Getenv("IAM_MOCK_COHORT_NATIVE_REDIS_URL"))
	if err != nil || opt.DB < 1 {
		t.Fatal("dedicated Redis DB >=1 required")
	}
	rh, _, err := net.SplitHostPort(opt.Addr)
	if err != nil || !net.ParseIP(rh).IsLoopback() {
		t.Fatal("native Redis must use explicit loopback")
	}
	rc := redis.NewClient(opt)
	defer func() { _ = rc.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	if n, er := rc.DBSize(ctx).Result(); er != nil || n != 0 {
		t.Fatalf("native Redis logical database must be empty: keys=%d err=%v", n, er)
	}
	id, err := readIdentity(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	id.RedisSHA, err = redisIdentity(ctx, rc)
	if err != nil {
		t.Fatal(err)
	}
	gdb, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("ProductionPreflightRejectsUnsettledBootstrap", func(t *testing.T) {
		if _, er := maintenanceServices(ctx, gdb, catalog); !errors.Is(er, eventoutbox.ErrUnsafeMessagingHandoff) {
			t.Fatalf("unsettled original bootstrap events must block production maintenance: %v", er)
		}
	})
	// Engine qualification only. The real production preflight above remains
	// blocked until an actual Relay/NSQ handoff settles bootstrap messages. No
	// legacy row is deleted, relabelled published or claimed as delivered here.
	cfg, err := eventcatalog.Parse(catalog)
	if err != nil {
		t.Fatal(err)
	}
	stager, err := eventoutbox.NewStandardStager(eventcatalog.NewCatalog(cfg))
	if err != nil {
		t.Fatal(err)
	}
	fixture := func(t *testing.T, base uint64) *engine {
		t.Helper()
		u, p, l := strconv.FormatUint(base, 10), strconv.FormatUint(base+1, 10), strconv.FormatUint(base+2, 10)
		exec := func(q string, a ...any) {
			t.Helper()
			if _, e := conn.ExecContext(ctx, q, a...); e != nil {
				t.Fatal(e)
			}
		}
		// Historical defaults must restore their captured values, not a new NOW().
		exec("INSERT INTO users(id,name,email,status,created_at,updated_at) VALUES(?,?,?,1,'2019-01-02 03:04:05','2019-02-03 04:05:06')", u, "dedicated native mock", u+"@example.invalid")
		exec("INSERT INTO profiles(id,name,gender,birthday) VALUES(?,?,0,NULL)", p, "native profile")
		exec("INSERT INTO profile_links(id,user_id,profile_id,type,relation,established_at) VALUES(?,?,?,'relation','other',UTC_TIMESTAMP())", base+3, u, p)
		exec("INSERT INTO auth_login_identities(id,user_id,provider,realm,identifier,status,linked_at) VALUES(?,?,'username','default',?,'active',UTC_TIMESTAMP())", l, u, "native-"+u)
		exec("INSERT INTO auth_credentials(id,login_identity_id,type,material,status) VALUES(?,?,'password',?,'active')", base+4, l, []byte{0, 255, 42})
		exec("INSERT INTO authz_roles(id,name,display_name,is_system,management_protection) VALUES(?,?,?,0,'standard')", base+5, "native:mock:"+u, "native mock role")
		exec("INSERT INTO authz_assignments(id,subject_type,subject_id,role_id,org_id,scope_kind,granted_by,granted_at) VALUES(?,'user',?,?,1,'all_stores','10001',UTC_TIMESTAMP())", base+6, u, base+5)
		s := iamScope{[]string{u}, []string{p}, []string{l}}
		tables, e := readScope(ctx, conn, s)
		if e != nil {
			t.Fatal(e)
		}
		if e = validateClosure(tables, s); e != nil {
			t.Fatal(e)
		}
		protected, e := readScope(ctx, conn, iamScope{[]string{"10001"}, []string{}, []string{}})
		if e != nil {
			t.Fatal(e)
		}
		d := t.TempDir()
		_ = os.Chmod(d, 0700)
		j, e := newJournal(filepath.Join(d, "apply.jsonl"), "native-"+u, strings.Repeat("a", 64))
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { _ = j.f.Close() })
		e1 := &engine{db: gdb, redis: rc, archive: archive{Format: 1, Identity: id, Tables: tables, Protected: protected}, scope: s, journal: j, stager: stager}
		store := sessioncache.NewSessionStore(rc)
		for _, sid := range []string{"indexed-" + u, "unindexed-" + u} {
			sess := &sessiondomain.Session{SessionID: sid, UserID: meta.FromUint64(base), LoginIdentityID: meta.FromUint64(base + 2), Status: sessiondomain.StatusActive, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
			if e = store.Save(ctx, sess); e != nil {
				t.Fatal(e)
			}
			if strings.HasPrefix(sid, "unindexed-") {
				if e = rc.ZRem(ctx, keyspace.New("user_session_index").Prefix(u), sid).Err(); e != nil {
					t.Fatal(e)
				}
				if e = rc.ZRem(ctx, keyspace.New("login_identity_session_index").Prefix(l), sid).Err(); e != nil {
					t.Fatal(e)
				}
			}
		}
		return e1
	}
	t.Run("ActiveAssignmentAndSessionsApplyRestore", func(t *testing.T) {
		e := fixture(t, 810000101)
		var before int64
		if err = conn.QueryRowContext(ctx, "SELECT COALESCE(MAX(policy_version),0) FROM authz_policy_versions").Scan(&before); err != nil {
			t.Fatal(err)
		}
		if err = e.apply(ctx); err != nil {
			t.Fatal(err)
		}
		if err = e.verify(ctx); err != nil {
			t.Fatal(err)
		}
		var after int64
		if err = conn.QueryRowContext(ctx, "SELECT MAX(policy_version) FROM authz_policy_versions").Scan(&after); err != nil {
			t.Fatal(err)
		}
		if after <= before {
			t.Fatal("revoke did not advance policy")
		}
		raw, _ := os.ReadFile(e.journal.f.Name())
		entries, er := parseJournal(raw, e.journal.operation, e.journal.archiveSHA)
		if er != nil {
			t.Fatal(er)
		}
		restoreJournal, er := newJournal(filepath.Join(filepath.Dir(e.journal.f.Name()), "restore.jsonl"), e.journal.operation, e.journal.archiveSHA)
		if er != nil {
			t.Fatal(er)
		}
		defer func() { _ = restoreJournal.f.Close() }()
		e.journal = restoreJournal
		if er = e.restore(ctx, entries); er != nil {
			t.Fatal(er)
		}
		var restoredVersion int64
		if err = conn.QueryRowContext(ctx, "SELECT MAX(policy_version) FROM authz_policy_versions").Scan(&restoredVersion); err != nil {
			t.Fatal(err)
		}
		if restoredVersion <= after {
			t.Fatal("restore rolled back or omitted policy event")
		}
		var events int
		if err = conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM rm_outbox WHERE event_type=?", eventing.AuthzVersionChanged).Scan(&events); err != nil {
			t.Fatal(err)
		}
		if events < 2 {
			t.Fatal("durable policy events absent")
		}
		store := sessioncache.NewSessionStore(rc)
		for _, sid := range []string{"indexed-810000101", "unindexed-810000101"} {
			s, er := store.Get(ctx, sid)
			if er != nil || s == nil || s.IsActive() {
				t.Fatal("SQL restore revived session")
			}
		}
	})
	t.Run("PreparedTransactionRollbackIsReadBack", func(t *testing.T) {
		e := fixture(t, 810000201)
		im := lookup(e.archive.Tables, "auth_credentials")
		calls := 0
		e.check = func(context.Context) error {
			calls++
			if calls > 1 {
				return errors.New("native cancellation before commit")
			}
			return nil
		}
		er := e.mutate(ctx, "delete", im, func(c context.Context, tx *gorm.DB) error { return deleteRows(c, tx, im, im.Rows) })
		if er == nil {
			t.Fatal("cancelled mutation succeeded")
		}
		got, er := currentRows(ctx, conn, im, im.Rows)
		if er != nil || !rowsEqual(got.Rows, im.Rows) {
			t.Fatal("rollback failed")
		}
		raw, _ := os.ReadFile(e.journal.f.Name())
		entries, er := parseJournal(raw, e.journal.operation, e.journal.archiveSHA)
		if er != nil || entries[len(entries)-1].Result != "readback_before" {
			t.Fatal("unknown readback was not preserved")
		}
	})
	t.Run("RestoreRejectsForeignReplacement", func(t *testing.T) {
		e := fixture(t, 810000301)
		if er := e.apply(ctx); er != nil {
			t.Fatal(er)
		}
		raw, _ := os.ReadFile(e.journal.f.Name())
		entries, er := parseJournal(raw, e.journal.operation, e.journal.archiveSHA)
		if er != nil {
			t.Fatal(er)
		}
		if _, er = conn.ExecContext(ctx, "INSERT INTO users(id,name,email,status) VALUES(810000301,'foreign','foreign@example.invalid',1)"); er != nil {
			t.Fatal(er)
		}
		if er = e.restore(ctx, entries); er == nil {
			t.Fatal("foreign replacement overwritten")
		}
		var name string
		if err = conn.QueryRowContext(ctx, "SELECT name FROM users WHERE id=810000301").Scan(&name); err != nil {
			t.Fatal(err)
		}
		if name != "foreign" {
			t.Fatal("foreign row changed")
		}
	})
	t.Run("RestoreConflictPreservesNewForeignSession", func(t *testing.T) {
		e := fixture(t, 810000501)
		if er := e.apply(ctx); er != nil {
			t.Fatal(er)
		}
		raw, er := os.ReadFile(e.journal.f.Name())
		if er != nil {
			t.Fatal(er)
		}
		entries, er := parseJournal(raw, e.journal.operation, e.journal.archiveSHA)
		if er != nil {
			t.Fatal(er)
		}
		if _, er = conn.ExecContext(ctx, "INSERT INTO users(id,name,email,status) VALUES(810000501,'new foreign user','new-foreign@example.invalid',1)"); er != nil {
			t.Fatal(er)
		}
		if _, er = conn.ExecContext(ctx, "INSERT INTO auth_login_identities(id,user_id,provider,realm,identifier,status,linked_at) VALUES(810000503,810000501,'username','default','new-foreign-login','active',UTC_TIMESTAMP())"); er != nil {
			t.Fatal(er)
		}
		foreignBefore, er := readScope(ctx, conn, e.scope)
		if er != nil {
			t.Fatal(er)
		}
		store := sessioncache.NewSessionStore(rc)
		sid := "new-foreign-session"
		s := &sessiondomain.Session{SessionID: sid, UserID: meta.FromUint64(810000501), LoginIdentityID: meta.FromUint64(810000503), Status: sessiondomain.StatusActive, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
		if er = store.Save(ctx, s); er != nil {
			t.Fatal(er)
		}
		key := keyspace.New("session").Prefix(sid)
		sessionBefore, er := rc.Get(ctx, key).Result()
		if er != nil {
			t.Fatal(er)
		}
		if er = e.restore(ctx, entries); er == nil {
			t.Fatal("foreign SQL replacement admitted restore")
		}
		sessionAfter, er := rc.Get(ctx, key).Result()
		if er != nil || sessionBefore != sessionAfter {
			t.Fatal("restore conflict changed the new foreign session before rejecting SQL")
		}
		active, er := store.Get(ctx, sid)
		if er != nil || active == nil || !active.IsActive() {
			t.Fatal("restore conflict revoked the new foreign session")
		}
		foreignAfter, er := readScope(ctx, conn, e.scope)
		if er != nil || !imageRowsEqual(foreignBefore, foreignAfter) {
			t.Fatal("restore conflict changed foreign SQL facts")
		}
	})
	t.Run("LockingPreimagePreventsConcurrentReplacement", func(t *testing.T) {
		e := fixture(t, 810000401)
		im := lookup(e.archive.Tables, "auth_credentials")
		other, er := sql.Open("mysql", dsn.FormatDSN())
		if er != nil {
			t.Fatal(er)
		}
		defer func() { _ = other.Close() }()
		done := make(chan error, 1)
		er = e.mutate(ctx, "delete", im, func(c context.Context, tx *gorm.DB) error {
			go func() {
				res, x := other.ExecContext(ctx, "UPDATE auth_credentials SET material=? WHERE id=?", []byte("new fact"), 810000405)
				if x == nil {
					var n int64
					n, x = res.RowsAffected()
					if x == nil && n != 0 {
						x = errors.New("concurrent replacement escaped row lock")
					}
				}
				done <- x
			}()
			select {
			case x := <-done:
				return errors.Join(errors.New("competing update did not wait for fixed-PK lock"), x)
			case <-time.After(100 * time.Millisecond):
			}
			return deleteRows(c, tx, im, im.Rows)
		})
		if er != nil {
			t.Fatal(er)
		}
		select {
		case x := <-done:
			if x != nil {
				t.Fatal(x)
			}
		case <-ctx.Done():
			t.Fatal("competing update did not finish")
		}
	})
}

func prepareNativeSchema(t *testing.T, dsn *driver.Config, catalogRaw []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	seedDSN := *dsn
	seedDSN.MultiStatements = true
	prepPool, err := sql.Open("mysql", seedDSN.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = prepPool.Close() }()
	var existing int
	if err = prepPool.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()").Scan(&existing); err != nil || existing != 0 {
		t.Fatalf("native schema must be empty: tables=%d err=%v", existing, err)
	}
	prep, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: prepPool}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := eventcatalog.Parse(catalogRaw)
	if err != nil {
		t.Fatal(err)
	}
	stager, err := eventoutbox.NewBootstrapStager(eventcatalog.NewCatalog(cfg))
	if err != nil {
		t.Fatal(err)
	}
	pool, err := sql.Open("mysql", seedDSN.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pool.Close() }()
	head, _, err := migration.NewMigrator(pool, &migration.Config{Enabled: true, Database: dsn.DBName, FreshStages: []migration.FreshStage{
		{Version: 31, Prepare: func() error { return rolemodel.PrepareBootstrapTenant(ctx, prep) }},
		{Version: 33, Prepare: func() error { return rolemodel.BootstrapIndependentRoles(ctx, prep, stager) }},
		{Version: 35, Prepare: func() error { return conditionretire.Bootstrap(ctx, prep, stager) }},
	}}).Run()
	if err != nil || head != 40 {
		t.Fatalf("original full migration/hooks failed: head=%d err=%v", head, err)
	}
}
