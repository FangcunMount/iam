//go:build integration

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	sessiondomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authn/session"
	sessioncache "github.com/FangcunMount/iam/v5/internal/apiserver/infra/cache/redis"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	driver "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestIAMMockCohortNative1000Performance(t *testing.T) {
	if os.Getenv("IAM_MOCK_COHORT_PERF_REQUIRED") != "1" {
		t.Skip("explicit isolated 1000-user performance lane required")
	}
	dsn, err := driver.ParseDSN(os.Getenv("IAM_MOCK_COHORT_NATIVE_MYSQL_DSN"))
	if err != nil || !strings.HasPrefix(dsn.DBName, "iam_mock_") || dsn.Net != "tcp" {
		t.Fatal("dedicated fresh native schema required")
	}
	host, _, err := net.SplitHostPort(dsn.Addr)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		t.Fatal("native MySQL must use explicit loopback")
	}
	dsn.ParseTime = true
	dsn.Loc = time.UTC
	dsn.MultiStatements = false
	opt, err := redis.ParseURL(os.Getenv("IAM_MOCK_COHORT_NATIVE_REDIS_URL"))
	if err != nil || opt.DB < 1 {
		t.Fatal("dedicated Redis DB>=1 required")
	}
	host, _, err = net.SplitHostPort(opt.Addr)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		t.Fatal("native Redis must use explicit loopback")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	rc := redis.NewClient(opt)
	defer func() { _ = rc.Close() }()
	if n, e := rc.DBSize(ctx).Result(); e != nil || n != 0 {
		t.Fatalf("fresh Redis DB required: keys=%d error=%v", n, e)
	}
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
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	gdb, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if _, e := maintenanceServices(ctx, gdb, catalog); !errors.Is(e, eventoutbox.ErrUnsafeMessagingHandoff) {
		t.Fatalf("production preflight must still reject original pending bootstrap messages: %v", e)
	}
	cfg, err := eventcatalog.Parse(catalog)
	if err != nil {
		t.Fatal(err)
	}
	stager, err := eventoutbox.NewStandardStager(eventcatalog.NewCatalog(cfg))
	if err != nil {
		t.Fatal(err)
	}
	type phase struct {
		Seconds float64 `json:"seconds"`
	}
	phases := map[string]phase{}
	measure := func(name string, fn func() error) {
		t.Helper()
		start := time.Now()
		if e := fn(); e != nil {
			t.Fatalf("phase %s: %v", name, e)
		}
		phases[name] = phase{time.Since(start).Seconds()}
	}
	scope := iamScope{Users: []string{}, Profiles: []string{}, LoginIdentities: []string{}}
	// Exactly 1000 dedicated users, each with one profile, identity, binary
	// credential, link and active session; no authorization assignments.
	const count = 1000
	for i := 0; i < count; i++ {
		base := uint64(820000000 + i*8)
		scope.Users = append(scope.Users, strconv.FormatUint(base, 10))
		scope.Profiles = append(scope.Profiles, strconv.FormatUint(base+1, 10))
		scope.LoginIdentities = append(scope.LoginIdentities, strconv.FormatUint(base+2, 10))
	}
	measure("seed", func() error {
		for _, table := range []string{"users", "profiles", "auth_login_identities", "auth_credentials", "profile_links"} {
			rows, args := []string{}, []any{}
			var cols string
			for i := 0; i < count; i++ {
				base := uint64(820000000 + i*8)
				u, p, l := scope.Users[i], scope.Profiles[i], scope.LoginIdentities[i]
				switch table {
				case "users":
					cols = "id,name,email,status,created_at,updated_at"
					rows = append(rows, "(?,?,?,1,'2019-01-02 03:04:05','2019-02-03 04:05:06')")
					args = append(args, u, "mock performance", u+"@example.invalid")
				case "profiles":
					cols = "id,name,gender,birthday"
					rows = append(rows, "(?,?,0,NULL)")
					args = append(args, p, "mock performance")
				case "auth_login_identities":
					cols = "id,user_id,provider,realm,identifier,status,linked_at"
					rows = append(rows, "(?,?,'username','default',?,'active','2019-01-02 03:04:05')")
					args = append(args, l, u, "mock-perf-"+u)
				case "auth_credentials":
					cols = "id,login_identity_id,type,material,status"
					rows = append(rows, "(?,?,'password',?,'active')")
					args = append(args, base+4, l, []byte{0, 255, 42})
				case "profile_links":
					cols = "id,user_id,profile_id,type,relation,established_at"
					rows = append(rows, "(?,?,?,'relation','other','2019-01-02 03:04:05')")
					args = append(args, base+3, u, p)
				}
			}
			if _, e := conn.ExecContext(ctx, "INSERT INTO "+quoted(table)+" ("+cols+") VALUES "+strings.Join(rows, ","), args...); e != nil {
				return e
			}
		}
		store := sessioncache.NewSessionStore(rc)
		for i, u := range scope.Users {
			uid, e := meta.ParseID(u)
			if e != nil {
				return e
			}
			lid, e := meta.ParseID(scope.LoginIdentities[i])
			if e != nil {
				return e
			}
			if e = store.Save(ctx, &sessiondomain.Session{SessionID: "mock-perf-" + u, UserID: uid, LoginIdentityID: lid, Status: sessiondomain.StatusActive, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}); e != nil {
				return e
			}
		}
		return nil
	})
	id, err := readIdentity(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	id.RedisSHA, err = redisIdentity(ctx, rc)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err = os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(dir, "archive.json")
	var a archive
	var archiveRaw []byte
	measure("capture", func() error {
		tx, e := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
		if e != nil {
			return e
		}
		defer func() { _ = tx.Rollback() }()
		tables, e := readScope(ctx, tx, scope)
		if e != nil {
			return e
		}
		if e = validateClosure(tables, scope); e != nil {
			return e
		}
		if e = validateExternal(ctx, tx, scope); e != nil {
			return e
		}
		protected, e := readScope(ctx, tx, iamScope{[]string{"10001"}, []string{}, []string{}})
		if e != nil {
			return e
		}
		if len(lookup(tables, "authz_assignments").Rows) != 0 {
			return errors.New("unexpected mock assignments")
		}
		a = archive{Format: 1, Kind: "mock_cohort_iam_archive", Operation: "native-perf-1000", Source: "native-engine-only", GroupSHA: strings.Repeat("a", 64), Identity: id, Tables: tables, Protected: protected}
		archiveRaw = encode(a)
		if e = tx.Commit(); e != nil {
			return e
		}
		return writeExclusive(archivePath, archiveRaw)
	})
	archiveSHA := rawSHA(archiveRaw)
	checkOriginal := func() error { _, e := readPrivate(archivePath, archiveSHA, maxDocument); return e }
	if err = checkOriginal(); err != nil {
		t.Fatal(err)
	}
	j, err := newJournal(filepath.Join(dir, "apply.jsonl"), a.Operation, archiveSHA)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = j.f.Close() }()
	e := engine{db: gdb, redis: rc, archive: a, scope: scope, journal: j, stager: stager}
	measure("apply", func() error { return e.apply(ctx) })
	measure("verify", func() error { return e.verify(ctx) })
	if err = checkOriginal(); err != nil {
		t.Fatal(err)
	}
	applyRaw, err := os.ReadFile(j.f.Name())
	if err != nil {
		t.Fatal(err)
	}
	entries, err := parseJournal(applyRaw, a.Operation, archiveSHA)
	if err != nil {
		t.Fatal(err)
	}
	restore, err := newJournal(filepath.Join(dir, "restore.jsonl"), a.Operation, archiveSHA)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = restore.f.Close() }()
	e.journal = restore
	measure("restore", func() error { return e.restore(ctx, entries) })
	if err = checkOriginal(); err != nil {
		t.Fatal(err)
	}
	restored, err := readScope(ctx, conn, scope)
	if err != nil || !imageRowsEqual(a.Tables, restored) {
		t.Fatalf("complete original values differ: %v", err)
	}
	rows := map[string]int{}
	for _, im := range a.Tables {
		rows[im.Name] = len(im.Rows)
	}
	report := struct {
		Kind         string           `json:"kind"`
		Users        int              `json:"users"`
		Rows         map[string]int   `json:"rows"`
		Sessions     int              `json:"seeded_active_sessions"`
		ArchiveBytes int              `json:"archive_bytes"`
		ArchiveSHA   string           `json:"archive_sha256"`
		JournalBytes int              `json:"apply_journal_bytes"`
		Phases       map[string]phase `json:"phases"`
		Boundary     string           `json:"boundary"`
	}{"iam_mock_cohort_1000_performance", count, rows, count, len(archiveRaw), archiveSHA, len(applyRaw), phases, "isolated engine-only phase timing; no SQL instrumentation; no production 30m/60m/RPO qualification"}
	t.Log(fmt.Sprintf("IAM_MOCK_COHORT_PERFORMANCE %s", encode(report)))
}
