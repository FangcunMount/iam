//go:build integration

package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	keyspace "github.com/FangcunMount/component-base/pkg/redis/keyspace"
	sessiondomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authn/session"
	sessioncache "github.com/FangcunMount/iam/v5/internal/apiserver/infra/cache/redis"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/FangcunMount/iam/v5/pkg/version"
	"github.com/FangcunMount/reliable-messaging/wire/legacy"
	driver "github.com/go-sql-driver/mysql"
	"github.com/nsqio/go-nsq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// This lane owns a fresh original 1..40 schema, empty Redis DB and isolated
// NSQD. Unlike the engine-only native lane, every cohort action below enters
// the real serve/execute CLI in a separate OS process. QS documents are explicit
// synthetic interface inputs, not evidence that this test ran a QS whole graph.
func TestIAMMockCohortNativeCLI(t *testing.T) {
	require.Equal(t, "1", os.Getenv("IAM_MOCK_COHORT_CLI_NATIVE_REQUIRED"), "explicit owned CLI lane required; no mandatory skip")
	require.Regexp(t, "^[0-9a-f]{40}$", version.GitCommit, "inject the actual archived test source")
	dsn, err := driver.ParseDSN(os.Getenv("IAM_MOCK_COHORT_NATIVE_MYSQL_DSN"))
	require.NoError(t, err)
	require.Equal(t, "tcp", dsn.Net)
	require.True(t, strings.HasPrefix(dsn.DBName, "iam_mock_"))
	cliNativeLoopback(t, dsn.Addr)
	dsn.ParseTime, dsn.Loc, dsn.MultiStatements = true, time.UTC, false
	dsn.Timeout, dsn.ReadTimeout, dsn.WriteTimeout = 5*time.Second, 15*time.Second, 15*time.Second
	redisURL := os.Getenv("IAM_MOCK_COHORT_NATIVE_REDIS_URL")
	ro, err := redis.ParseURL(redisURL)
	require.NoError(t, err)
	require.Greater(t, ro.DB, 0)
	cliNativeLoopback(t, ro.Addr)
	cliNativeLoopback(t, os.Getenv("RM_IAM_NSQ_TCP"))
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	rc := redis.NewClient(ro)
	t.Cleanup(func() { require.NoError(t, rc.Close()) })
	n, err := rc.DBSize(ctx).Result()
	require.NoError(t, err)
	require.Zero(t, n, "never flush or reuse a populated Redis DB")
	catalogRaw, err := os.ReadFile(filepath.Join("..", "..", "..", "configs", "events.yaml"))
	require.NoError(t, err)
	prepareNativeSchema(t, dsn, catalogRaw)
	db, err := sql.Open("mysql", dsn.FormatDSN())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	gdb, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), DisableAutomaticPing: true})
	require.NoError(t, err)
	if !t.Run("OriginalBootstrapDeliveredByPinnedLegacyRelay", func(t *testing.T) {
		_, er := maintenanceServices(ctx, gdb, catalogRaw)
		require.ErrorIs(t, er, eventoutbox.ErrUnsafeMessagingHandoff)
		cliNativeDrain(t, ctx, dsn, gdb, catalogRaw)
		_, er = maintenanceServices(ctx, gdb, catalogRaw)
		require.NoError(t, er, "actual legacy delivery must satisfy the unchanged production gate")
	}) {
		return
	}
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	dir, err := os.MkdirTemp(home, ".iam-mock-cli-native-")
	require.NoError(t, err)
	require.NoError(t, os.Chmod(dir, 0700))
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(dir)) })
	f := &cliNativeFixture{ctx: ctx, dir: dir, dsn: dsn.FormatDSN(), redisURL: redisURL, files: map[string]inputFile{}}
	scope := cliNativeSeed(t, ctx, conn, rc, 830000101)
	retained := cliNativeSeed(t, ctx, conn, rc, 830000201)
	protectedRaw := encode(protection{1, "mock_cohort_protected", retained, json.RawMessage(`{"system":[],"clack":[],"other":[]}`)})
	qs := json.RawMessage(`{"selector":{"source":"native_cli_synthetic","org_id":"1"}}`)
	ownerRaw := encode(ownerConfirmation{1, "mock_cohort_owner_confirmation", "native-cli-cycle", "10001", jsonSHA(scope), rawSHA(qs), rawSHA(protectedRaw), true, true, []string{"1"}})
	g := groupManifest{1, "mock_cohort", "native-cli-cycle", rawSHA(ownerRaw), rawSHA(protectedRaw), mustMarshal(scope), qs}
	f.put(t, "group", encode(g))
	f.put(t, "owner-confirmation", ownerRaw)
	f.put(t, "protected", protectedRaw)
	f.put(t, "event-catalog", catalogRaw)
	f.files["archive"] = inputFile{path: filepath.Join(dir, "archive.json")}
	var a archive
	if !t.Run("PlanAndCaptureThroughRealCLIProcesses", func(t *testing.T) {
		f.success(t, "plan", f.args("plan"))
		r := f.success(t, "capture", f.args("capture"))
		f.files["archive"] = inputFile{f.files["archive"].path, r.ArchiveSHA}
		raw, er := load(f.files["archive"])
		require.NoError(t, er)
		require.NoError(t, strictJSON(raw, &a))
		require.Equal(t, version.GitCommit, a.Source)
		require.Equal(t, scope.counts(), r.Counts)
		for _, name := range tableOrder {
			require.Len(t, lookup(a.Tables, name).Rows, 1, "all six physical scopes must be real: %s", name)
		}
	}) {
		return
	}
	f.prepareWrite(t, g, scope, a)
	if !t.Run("PendingAndPartialQSOriginalsRejectBeforeCredentials", func(t *testing.T) {
		original := f.files["qs-apply"]
		valid, er := load(original)
		require.NoError(t, er)
		var qr qsActionReceipt
		require.NoError(t, strictJSON(valid, &qr))
		for _, pending := range []bool{true, false} {
			bad := qr
			bad.SemanticPending, bad.Whole = pending, false
			f.put(t, "qs-apply", encode(bad))
			// Rebind the raw document into the handoff and approval. This tests the
			// semantic gate, not a trivially stale SHA before it.
			f.bindHandoff(t, g, scope, a, "apply", "")
			calls := 0
			var out bytes.Buffer
			code := serve(f.args("apply"), &out, func(string) string { calls++; return "" })
			var r receipt
			require.NoError(t, strictJSON(out.Bytes(), &r))
			require.Equal(t, 1, code)
			require.False(t, r.Complete)
			require.Zero(t, calls, "invalid QS completion must not open SQL/Redis")
			require.False(t, r.SQLClosed || r.RedisClosed || r.GateReleased)
			_, er = os.Stat(f.files["archive"].path + ".apply-claim")
			require.True(t, os.IsNotExist(er), "admission failure must not consume apply")
		}
		f.files["qs-apply"] = original
		f.bindHandoff(t, g, scope, a, "apply", "")
	}) {
		return
	}
	var applyRaw []byte
	if !t.Run("ApplyVerifyAndForeignRestoreConflictThroughCLI", func(t *testing.T) {
		f.files["journal"] = inputFile{path: filepath.Join(dir, "apply.jsonl")}
		r := f.success(t, "apply", f.args("apply"))
		var er error
		applyRaw, er = readPrivate(f.files["journal"].path, r.JournalSHA, maxDocument)
		require.NoError(t, er)
		entries, er := parseJournal(applyRaw, g.Operation, f.files["archive"].sha)
		require.NoError(t, er)
		require.NotEmpty(t, entries)
		require.Equal(t, "complete", entries[len(entries)-1].Result)
		f.success(t, "verify", f.args("verify"))
		// A foreign row at the original physical PK must survive a fresh CLI
		// restore attempt; the test removes only this exact synthetic replacement.
		_, er = conn.ExecContext(ctx, "INSERT INTO auth_credentials(id,login_identity_id,type,material,status) VALUES(830000105,830000103,'password',?,'active')", []byte("foreign-native-replacement"))
		require.NoError(t, er)
		f.files["prior-journal"] = inputFile{f.files["journal"].path, rawSHA(applyRaw)}
		f.files["journal"] = inputFile{path: filepath.Join(dir, "conflict-restore.jsonl")}
		f.bindHandoff(t, g, scope, a, "restore", rawSHA(applyRaw))
		got, code := f.invoke(t, f.args("restore"))
		require.Equal(t, 1, code)
		require.False(t, got.Complete)
		require.Equal(t, "operation_incomplete", got.Error)
		cliNativeReleased(t, got)
		var material []byte
		require.NoError(t, conn.QueryRowContext(ctx, "SELECT material FROM auth_credentials WHERE id=830000105").Scan(&material))
		require.Equal(t, []byte("foreign-native-replacement"), material)
		res, er := conn.ExecContext(ctx, "DELETE FROM auth_credentials WHERE id=830000105 AND BINARY material=?", material)
		require.NoError(t, er)
		n, er := res.RowsAffected()
		require.NoError(t, er)
		require.EqualValues(t, 1, n)
	}) {
		return
	}
	t.Run("FreshProcessRestoresOriginalJournalWithoutRevivingSessions", func(t *testing.T) {
		f.files["journal"] = inputFile{path: filepath.Join(dir, "fresh-restore.jsonl")}
		f.bindHandoff(t, g, scope, a, "restore", rawSHA(applyRaw))
		f.success(t, "restore", f.args("restore"))
		got, er := readScope(ctx, conn, scope)
		require.NoError(t, er)
		require.True(t, imageRowsEqual(a.Tables, got), "all captured original values including binary/NULL/historical timestamps must return")
		protected, er := readScope(ctx, conn, iamScope{appendUnique(retained.Users, "10001", "618855329043591726"), appendUnique(retained.Profiles, "632367179001508398", "636374014138266158"), retained.LoginIdentities})
		require.NoError(t, er)
		require.True(t, imageRowsEqual(a.Protected, protected), "real/shared and default protected facts changed")
		prior, er := load(f.files["prior-journal"])
		require.NoError(t, er)
		require.Equal(t, applyRaw, prior, "new CLI processes must read, not rewrite the original journal")
		for _, sid := range []string{"indexed-830000101", "unindexed-830000101"} {
			s, e := sessioncache.NewSessionStore(rc).Get(ctx, sid)
			require.NoError(t, e)
			require.NotNil(t, s)
			require.False(t, s.IsActive(), "SQL restore must never revive historical sessions")
		}
		keep, er := sessioncache.NewSessionStore(rc).Get(ctx, "indexed-830000201")
		require.NoError(t, er)
		require.NotNil(t, keep)
		require.True(t, keep.IsActive(), "retained user's session changed")
		var events int
		require.NoError(t, conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM rm_outbox WHERE event_type='iam.authz.version_changed.v2'").Scan(&events))
		require.GreaterOrEqual(t, events, 2, "revoke/restore must retain durable policy events")
		t.Log("true serve/execute subprocess cycle completed; synthetic QS handoff is interface-only, no production or QS whole-cohort qualification")
	})
	if !t.Failed() {
		t.Run("BatchClearanceV2ThroughRealCLI", func(t *testing.T) {
			cliNativeClearanceCycle(t, ctx, conn, rc, f, scope, retained, catalogRaw)
		})
	}
}

// Only the parent selects this helper. It returns the real CLI exit status and
// unmodified JSON stdout, so restore reloads persisted originals in a new process.
func TestIAMMockCohortNativeCLIChild(t *testing.T) {
	if os.Getenv("IAM_MOCK_COHORT_NATIVE_CLI_CHILD") != "1" {
		return
	}
	var args []string
	raw := os.Getenv("IAM_MOCK_COHORT_NATIVE_CLI_ARGS")
	if len(raw) > 32<<10 || json.Unmarshal([]byte(raw), &args) != nil {
		os.Exit(2)
	}
	os.Exit(serve(args, os.Stdout, os.Getenv))
}

type cliNativeFixture struct {
	ctx      context.Context
	dir, dsn string
	redisURL string
	files    map[string]inputFile
	seq      int
}

func (f *cliNativeFixture) put(t *testing.T, name string, raw []byte) {
	t.Helper()
	f.seq++
	p := filepath.Join(f.dir, fmt.Sprintf("%03d-%s.json", f.seq, name))
	require.NoError(t, writeExclusive(p, raw))
	f.files[name] = inputFile{p, rawSHA(raw)}
}
func (f *cliNativeFixture) args(action string) []string {
	out := []string{"--action=" + action, "--deadline=30s"}
	for _, name := range []string{"group", "owner-confirmation", "protected", "archive", "approval", "qs-handoff", "frozen", "event-catalog", "journal", "prior-journal", "qs-capture", "qs-apply", "qs-verify", "messages-settled"} {
		in, ok := f.files[name]
		if ok {
			out = append(out, "--"+name+"-file="+in.path, "--"+name+"-sha256="+in.sha)
		}
	}
	return out
}
func (f *cliNativeFixture) invoke(t *testing.T, args []string) (receipt, int) {
	t.Helper()
	exe, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(f.ctx, 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestIAMMockCohortNativeCLIChild$", "-test.v=false", "-test.timeout=40s")
	cmd.Env = cliNativeEnv(os.Environ(), map[string]string{"IAM_MOCK_COHORT_NATIVE_CLI_CHILD": "1", "IAM_MOCK_COHORT_NATIVE_CLI_ARGS": string(mustMarshal(args)), "IAM_MOCK_CLEANUP_MYSQL_DSN": f.dsn, "IAM_MOCK_CLEANUP_REDIS_URL": f.redisURL})
	var out, diagnostic cliNativeOutput
	cmd.Stdout, cmd.Stderr = &out, &diagnostic
	err = cmd.Run()
	if err != nil {
		var exit *exec.ExitError
		require.ErrorAs(t, err, &exit, "child did not return a real CLI exit status")
	}
	require.NoError(t, ctx.Err(), "CLI deadline is not an ordinary negative receipt")
	require.False(t, out.overflow)
	require.False(t, diagnostic.overflow)
	t.Logf("actual CLI stderr_sha256=%s", rawSHA(diagnostic.Bytes()))
	var r receipt
	require.NoError(t, strictJSON(out.Bytes(), &r), "invalid CLI stdout sha=%s", rawSHA(out.Bytes()))
	require.Equal(t, "mock_cohort_iam_receipt", r.Kind)
	require.Equal(t, version.GitCommit, r.Source)
	require.NotNil(t, cmd.ProcessState)
	require.True(t, cmd.ProcessState.Exited())
	f.put(t, "actual-cli-stdout", out.Bytes())
	t.Logf("actual CLI action=%s exit=%d stdout_sha256=%s stdout_base64=%s", r.Action, cmd.ProcessState.ExitCode(), rawSHA(out.Bytes()), base64.StdEncoding.EncodeToString(out.Bytes()))
	return r, cmd.ProcessState.ExitCode()
}
func (f *cliNativeFixture) success(t *testing.T, action string, args []string) receipt {
	t.Helper()
	r, code := f.invoke(t, args)
	require.Equal(t, 0, code)
	require.Equal(t, action, r.Action)
	require.True(t, r.Complete)
	require.Empty(t, r.Error)
	cliNativeReleased(t, r)
	return r
}
func cliNativeReleased(t *testing.T, r receipt) {
	t.Helper()
	require.True(t, r.SQLClosed && r.RedisClosed && r.GateReleased, "CLI must close both clients and release its actual database gate")
}
func (f *cliNativeFixture) prepareWrite(t *testing.T, g groupManifest, scope iamScope, a archive) {
	t.Helper()
	f.put(t, "frozen", encode(freezeReceipt{1, "mock_cohort_frozen", g.Operation, f.files["group"].sha, true, true, true, true}))
	f.put(t, "messages-settled", []byte("{\"kind\":\"synthetic_native_QS_handoff_only\"}\n"))
	for _, action := range []string{"capture", "apply", "verify"} {
		f.put(t, "qs-"+action, encode(qsActionReceipt{1, "mock_cohort_qs_action_result", action, g.Operation, f.files["group"].sha, strings.Repeat("b", 40), strings.Repeat("c", 64), true, false, action != "capture", false, 0, true}))
	}
	f.bindHandoff(t, g, scope, a, "apply", "")
}
func (f *cliNativeFixture) bindHandoff(t *testing.T, g groupManifest, scope iamScope, a archive, action, priorSHA string) {
	t.Helper()
	h := qsHandoff{Format: 1, Kind: "mock_cohort_qs_cleared", Operation: g.Operation, GroupSHA: f.files["group"].sha, OwnerSHA: g.OwnerSHA, ProtectedSHA: g.ProtectedSHA, IAMSHA: rawSHA(g.IAM), QSSHA: rawSHA(g.QS), IAMCounts: scope.counts(), CaptureSHA: f.files["qs-capture"].sha, ApplySHA: f.files["qs-apply"].sha, VerifySHA: f.files["qs-verify"].sha, SQLIdentity: json.RawMessage(`{"fixture":"synthetic_qs_sql_interface_only"}`), MongoIdentity: json.RawMessage(`{"fixture":"synthetic_qs_mongo_interface_only"}`), SQLVersion: 101, MongoVersion: 39, ReferencesClear: true, FrozenSHA: f.files["frozen"].sha, MessagesSHA: f.files["messages-settled"].sha}
	f.put(t, "qs-handoff", encode(h))
	now := time.Now().UTC()
	f.put(t, "approval", encode(approval{1, "mock_cohort_iam_approval", action, g.Operation, version.GitCommit, f.files["group"].sha, f.files["archive"].sha, f.files["qs-handoff"].sha, f.files["frozen"].sha, f.files["event-catalog"].sha, priorSHA, a.Identity, now, now.Add(4 * time.Minute), now.Add(4 * time.Minute)}))
}

func cliNativeSeed(t *testing.T, ctx context.Context, conn *sql.Conn, rc *redis.Client, base uint64) iamScope {
	t.Helper()
	u, p, l := strconv.FormatUint(base, 10), strconv.FormatUint(base+1, 10), strconv.FormatUint(base+2, 10)
	queries := []struct {
		q string
		a []any
	}{
		{"INSERT INTO users(id,name,email,status,created_at,updated_at) VALUES(?,?,?,1,'2019-01-02 03:04:05','2019-02-03 04:05:06')", []any{u, "CLI native dedicated", u + "@example.invalid"}},
		{"INSERT INTO profiles(id,name,gender,birthday) VALUES(?,?,0,NULL)", []any{p, "CLI native profile"}},
		{"INSERT INTO profile_links(id,user_id,profile_id,type,relation,established_at) VALUES(?,?,?,'relation','other',UTC_TIMESTAMP())", []any{base + 3, u, p}},
		{"INSERT INTO auth_login_identities(id,user_id,provider,realm,identifier,status,linked_at) VALUES(?,?,'username','default',?,'active',UTC_TIMESTAMP())", []any{l, u, "cli-native-" + u}},
		{"INSERT INTO auth_credentials(id,login_identity_id,type,material,status) VALUES(?,?,'password',?,'active')", []any{base + 4, l, []byte{0, 255, 42}}},
		{"INSERT INTO authz_roles(id,name,display_name,is_system,management_protection) VALUES(?,?,?,0,'standard')", []any{base + 5, "native:cli:" + u, "CLI native role"}},
		{"INSERT INTO authz_assignments(id,subject_type,subject_id,role_id,org_id,scope_kind,granted_by,granted_at) VALUES(?,'user',?,?,1,'all_stores','10001',UTC_TIMESTAMP())", []any{base + 6, u, base + 5}},
	}
	for _, q := range queries {
		_, err := conn.ExecContext(ctx, q.q, q.a...)
		require.NoError(t, err)
	}
	for _, prefix := range []string{"indexed-", "unindexed-"} {
		sid := prefix + u
		s := &sessiondomain.Session{SessionID: sid, UserID: meta.FromUint64(base), LoginIdentityID: meta.FromUint64(base + 2), Status: sessiondomain.StatusActive, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
		require.NoError(t, sessioncache.NewSessionStore(rc).Save(ctx, s))
		if prefix == "unindexed-" {
			require.NoError(t, rc.ZRem(ctx, keyspace.New("user_session_index").Prefix(u), sid).Err())
			require.NoError(t, rc.ZRem(ctx, keyspace.New("login_identity_session_index").Prefix(l), sid).Err())
		}
	}
	return iamScope{[]string{u}, []string{p}, []string{l}}
}

// Same protocol as cmd/iam-maintenance/reliable_messaging_bootstrap_test.go.
// The isolated runner archived a929f5301ed8265a8f44726b474da27ae10cb1c1 and built
// this exact Darwin ARM64 historical test binary. Its source/build receipt is
// an external runner input; the digest alone is not a source signature.
func cliNativeDrain(t *testing.T, ctx context.Context, dsn *driver.Config, db *gorm.DB, catalog []byte) {
	t.Helper()
	binary := os.Getenv("IAM_MOCK_COHORT_NATIVE_LEGACY_BINARY")
	require.True(t, filepath.IsAbs(binary))
	st, err := os.Lstat(binary)
	require.NoError(t, err)
	require.True(t, st.Mode().IsRegular())
	raw, err := os.ReadFile(binary)
	require.NoError(t, err)
	require.Equal(t, "f8c1704a5be4bf39ff50cb81cbf36b2f4f18dff353779f49cf42ea6e7b213f72", rawSHA(raw))
	var initial []eventoutbox.OutboxPO
	require.NoError(t, db.WithContext(ctx).Order("id").Find(&initial).Error)
	require.Len(t, initial, 2, "genuine original migration 33/35 bootstrap notifications")
	expected := map[string]string{}
	for _, row := range initial {
		require.Equal(t, "pending", row.Status)
		require.Equal(t, initial[0].TopicName, row.TopicName)
		expected[row.EventID] = row.PayloadJSON
	}
	require.Len(t, expected, 2)
	consumer, err := nsq.NewConsumer(initial[0].TopicName, "iam-mock-cli-"+rawSHA([]byte(dsn.DBName))[:12], nsq.NewConfig())
	require.NoError(t, err)
	consumer.SetLogger(nil, nsq.LogLevelError)
	received := make(chan []byte, 16)
	consumer.AddHandler(nsq.HandlerFunc(func(m *nsq.Message) error {
		select {
		case received <- append([]byte(nil), m.Body...):
		default:
		}
		return nil // real NSQ consumer FIN, not a fabricated database status
	}))
	require.NoError(t, consumer.ConnectToNSQD(os.Getenv("RM_IAM_NSQ_TCP")))
	t.Cleanup(func() {
		consumer.Stop()
		select {
		case <-consumer.StopChan:
		case <-time.After(5 * time.Second):
			t.Error("owned NSQ consumer release unproven")
		}
	})
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	dir, err := os.MkdirTemp(home, ".iam-mock-cli-drain-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(dir)) })
	catalogPath := filepath.Join(dir, "events.yaml")
	require.NoError(t, writeExclusive(catalogPath, catalog))
	host, port, err := net.SplitHostPort(dsn.Addr)
	require.NoError(t, err)
	childCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(childCtx, binary, "-test.run=^TestMaintenanceBootstrapHandoffChild$", "-test.v=false", "-test.timeout=25s")
	cmd.Env = cliNativeEnv(os.Environ(), map[string]string{"IAM_RM_BOOTSTRAP_REQUIRED": "1", "IAM_RM_HANDOFF_PHASE": "legacy-drain", "RM_IAM_NSQ_TCP": os.Getenv("RM_IAM_NSQ_TCP"), "RM_IAM_EVENTS_CATALOG": catalogPath, "IAM_APISERVER_MYSQL_HOST": host, "IAM_APISERVER_MYSQL_PORT": port, "IAM_APISERVER_MYSQL_USERNAME": dsn.User, "IAM_APISERVER_MYSQL_PASSWORD": dsn.Passwd, "IAM_APISERVER_MYSQL_DATABASE": dsn.DBName})
	var output cliNativeOutput
	cmd.Stdout, cmd.Stderr = &output, &output
	err = cmd.Run()
	require.NoError(t, err, "pinned legacy relay failed; bounded stdout_sha256=%s", rawSHA(output.Bytes()))
	require.NoError(t, childCtx.Err())
	require.False(t, output.overflow)
	require.NotNil(t, cmd.ProcessState)
	require.True(t, cmd.ProcessState.Exited())
	require.Equal(t, 0, cmd.ProcessState.ExitCode())
	seen := map[string]bool{}
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for len(seen) != len(expected) {
		select {
		case body := <-received:
			d, ok, e := legacy.Decode(body)
			require.NoError(t, e)
			require.True(t, ok)
			payload, exists := expected[d.UUID]
			require.True(t, exists, "foreign event must not settle this handoff")
			require.Equal(t, payload, string(d.Payload))
			require.Equal(t, "iam-outbox-relay", d.Metadata["source"])
			require.Equal(t, "iam.authz.version_changed.v2", d.Metadata["event_type"])
			seen[d.UUID] = true
		case <-timer.C:
			t.Fatal("real bootstrap NSQ delivery timed out")
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	require.NoError(t, eventoutbox.CheckLegacyDrained(ctx, db))
	var after []eventoutbox.OutboxPO
	require.NoError(t, db.WithContext(ctx).Order("id").Find(&after).Error)
	require.Len(t, after, len(initial))
	for i, row := range after {
		require.Equal(t, "published", row.Status)
		require.Equal(t, initial[i].ID, row.ID)
		require.Equal(t, initial[i].EventID, row.EventID)
		require.Equal(t, initial[i].PayloadJSON, row.PayloadJSON)
		require.Equal(t, initial[i].CreatedAt, row.CreatedAt)
	}
	t.Logf("actual prior source=a929f5301ed8265a8f44726b474da27ae10cb1c1 binary_sha256=%s delivered=%d NSQ bodies and original legacy rows retained", rawSHA(raw), len(seen))
}
func cliNativeLoopback(t *testing.T, address string) {
	t.Helper()
	host, _, err := net.SplitHostPort(address)
	require.NoError(t, err)
	ip := net.ParseIP(host)
	require.NotNil(t, ip)
	require.True(t, ip.IsLoopback(), "only dedicated loopback native dependencies are admitted")
}
func cliNativeEnv(base []string, replace map[string]string) []string {
	out := make([]string, 0, len(base)+len(replace))
	for _, s := range base {
		key, _, _ := strings.Cut(s, "=")
		if _, found := replace[key]; !found {
			out = append(out, s)
		}
	}
	for k, v := range replace {
		out = append(out, k+"="+v)
	}
	return out
}

type cliNativeOutput struct {
	bytes.Buffer
	overflow bool
}

func (w *cliNativeOutput) Write(p []byte) (int, error) {
	n := len(p)
	if remain := (64 << 10) - w.Len(); len(p) > remain {
		p, w.overflow = p[:remain], true
	}
	_, _ = w.Buffer.Write(p)
	return n, nil
}
