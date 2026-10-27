//go:build integration

package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	change "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/policychange"
	appuow "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/uow"
	domain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	policyrepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/policy"
	rolepo "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/role"
	hostuow "github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/uow/authz"
	dbmysql "github.com/FangcunMount/iam/v5/internal/pkg/database/mysql"
	txmysql "github.com/FangcunMount/reliable-messaging/storage/transaction/mysql"
	"github.com/FangcunMount/reliable-messaging/transaction"
	driver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestM3PlannedPolicyNative(t *testing.T) {
	dsn := os.Getenv("IAM_M3_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Fatal("owned disposable IAM_M3_TEST_MYSQL_DSN is required")
	}
	config, err := driver.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid disposable DSN")
	}
	config.DBName = ""
	config.MultiStatements = true
	config.ParseTime = true
	server, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	name := fmt.Sprintf("iam_m3_policy_test_%d", time.Now().UnixNano())
	if _, err = server.ExecContext(t.Context(), "CREATE DATABASE `"+name+"`"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := server.ExecContext(cleanup, "DROP DATABASE `"+name+"`"); err != nil {
			t.Error("owned planned-policy schema cleanup failed")
		}
	}()
	config.DBName = name
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	host, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: db}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// The migration command owns/closes its driver pool. The runtime pool and
	// preparation UOW are separate host resources, as in the full-chain fixture.
	migrationPool, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer migrationPool.Close()
	version, _, err := NewMigrator(migrationPool, &Config{Enabled: true, Database: name, FreshStages: freshStagesForTest(t, host)}).Run()
	if err != nil || version != 45 {
		t.Fatal("full original host schema", version, err)
	}
	if err = db.PingContext(t.Context()); err != nil {
		t.Fatal("borrowed host pool unavailable after explicit migration", err)
	}
	base := policyrepo.NewPolicyVersionRepository(host)
	current, err := base.GetCurrent(t.Context())
	if err != nil || current == nil {
		t.Fatal("original policy fact missing", err)
	}
	boundary := hostuow.NewUnitOfWork(host, nil)
	reader := boundary.(appuow.ReadOnlyPlanner)
	snapshot, err := appuow.PrepareChange(t.Context(), reader, appuow.WriteRequest{Operation: "role.update", Scope: "global", Preview: func(ctx context.Context, repos appuow.TxRepositories) (bool, error) {
		if repos.Events != nil {
			return false, errors.New("read planning exposed an event stager")
		}
		tx, err := dbmysql.RequireTx(ctx)
		if err != nil {
			return false, err
		}
		raw, ok := tx.Statement.ConnPool.(*sql.Tx)
		if !ok {
			return false, errors.New("planning root missing")
		}
		_, err = raw.ExecContext(ctx, `UPDATE authz_policy_versions SET policy_version=policy_version+1 WHERE id=1`)
		var denied *driver.MySQLError
		if !errors.As(err, &denied) || denied.Number != 1792 {
			return false, errors.New("database did not enforce read-only planning")
		}
		return false, nil
	}})
	if err != nil || snapshot.Changed || snapshot.Expected != current.Version {
		t.Fatal("read-only no-op plan", snapshot, err)
	}
	err = boundary.WithinTx(t.Context(), func(active context.Context, _ appuow.TxRepositories) error {
		return reader.ReadPlan(active, func(context.Context, appuow.TxRepositories) error { return errors.New("nested planning callback ran") })
	})
	if !errors.Is(err, dbmysql.ErrActiveTransactionRequired) {
		t.Fatal("planning borrowed an existing snapshot", err)
	}
	binding := transaction.Binding{Owner: "iam.mysql", StoreID: "owned-primary", WriterEpoch: "m3-1"}
	store, err := txmysql.New(db, binding)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(t.Context(), `INSERT INTO rm_tx_binding VALUES(?,?,?,1)`, []byte(binding.Owner), []byte(binding.StoreID), []byte(binding.WriterEpoch)); err != nil {
		t.Fatal(err)
	}
	selectOwnedTransactionProtocol(t, db, binding)
	planned := base.(domain.PlannedRepository)
	at := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	if _, err = planned.IncrementExpected(t.Context(), current.Version, "system", "reject pool", at); !errors.Is(err, dbmysql.ErrActiveTransactionRequired) {
		t.Fatal("ordinary DB handle accepted", err)
	}
	const concurrency = 8
	executions := make([]transaction.Execution, concurrency)
	for i := range concurrency {
		id := fmt.Sprintf("cas-%d", i)
		intent, err := change.NewCommandIntent("role.update", "user:7", "global", id, map[string]string{"id": "42", "display_name": "冻结名称"})
		if err != nil {
			t.Fatal(err)
		}
		plan, err := change.FreezeVersionPlan(intent, current.Version, id, "event-"+id, at, "iam.authz.version.v2", "m3.policy.test")
		if err != nil {
			t.Fatal(err)
		}
		reserved, err := store.Reserve(t.Context(), transaction.RequestFor(plan), plan, transaction.ReserveOptions{Lease: time.Minute})
		if err != nil || !reserved.Acquired {
			t.Fatal(err)
		}
		executions[i] = reserved.Execution
	}
	type outcome struct {
		index     int
		committed bool
		err       error
	}
	results := make(chan outcome, concurrency)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, e := range executions {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			raw, err := db.BeginTx(ctx, nil)
			if err != nil {
				results <- outcome{i, false, err}
				return
			}
			defer raw.Rollback()
			guard, err := store.Bind(ctx, raw, e)
			if err != nil {
				results <- outcome{i, false, err}
				return
			}
			tx, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: raw, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true, SkipDefaultTransaction: true})
			if err != nil {
				results <- outcome{i, false, err}
				return
			}
			intent, intentErr := change.NewCommandIntent("role.update", "user:7", "global", fmt.Sprintf("cas-%d", i), map[string]string{"id": "42", "display_name": "冻结名称"})
			if intentErr != nil {
				results <- outcome{i, false, intentErr}
				return
			}
			plannedCtx, intentErr := change.WithExecutionPlan(ctx, e, intent)
			if intentErr != nil {
				results <- outcome{i, false, intentErr}
				return
			}
			repo := policyrepo.NewPolicyVersionRepository(tx)
			// More than512 UTF-8 bytes, within the original VARCHAR(512) character limit.
			_, err = repo.Increment(plannedCtx, "system", strings.Repeat("中", 200))
			if errors.Is(err, domain.ErrStaleVersion) {
				if err = raw.Rollback(); err == nil {
					var ok bool
					ok, err = store.Abort(ctx, e)
					if !ok && err == nil {
						err = errors.New("stale execution did not persist abort")
					}
				}
				results <- outcome{i, false, err}
				return
			}
			if err == nil {
				err = guard.CommitWitness(ctx)
			}
			if err == nil {
				err = raw.Commit()
			}
			results <- outcome{i, err == nil, err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	winners := 0
	checker := transaction.StoreChecker{Store: store}
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		want := transaction.Rollback
		if result.committed {
			winners++
			want = transaction.Commit
		}
		if checker.Check(t.Context(), executions[result.index].References()[0]) != want {
			t.Fatal("CAS fact and witness diverged")
		}
	}
	if winners != 1 {
		t.Fatalf("%d effective executions, want1", winners)
	}
	after, err := base.GetCurrent(t.Context())
	if err != nil || after.Version != current.Version+1 {
		t.Fatal("stale plan chose another version", after, err)
	}
	// Explicit rollback keeps both the original version and all witnesses unchanged.
	id := "rollback-cas"
	intent, err := change.NewCommandIntent("role.update", "user:7", "global", id, map[string]string{"id": "42"})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := change.FreezeVersionPlan(intent, after.Version, id, "event-"+id, at, "iam.authz.version.v2", "m3.policy.test")
	if err != nil {
		t.Fatal(err)
	}
	r, err := store.Reserve(t.Context(), transaction.RequestFor(plan), plan, transaction.ReserveOptions{Lease: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Rollback()
	guard, err := store.Bind(t.Context(), raw, r.Execution)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: raw, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = policyrepo.NewPolicyVersionRepository(tx).(domain.PlannedRepository).IncrementExpected(t.Context(), after.Version, "system", "rollback", at)
	if err != nil {
		t.Fatal(err)
	}
	if err = guard.CommitWitness(t.Context()); err != nil {
		t.Fatal(err)
	}
	if checker.Check(t.Context(), r.Execution.References()[0]) != transaction.Unknown {
		t.Fatal("uncommitted root was visible")
	}
	if err = raw.Rollback(); err != nil {
		t.Fatal(err)
	}
	restored, err := base.GetCurrent(t.Context())
	if err != nil || restored.Version != after.Version {
		t.Fatal("root rollback lost", err)
	}
	if checker.Check(t.Context(), r.Execution.References()[0]) != transaction.Unknown {
		t.Fatal("rollback fabricated witness")
	}
	t.Run("guarded-host-root", func(t *testing.T) { testM3GuardedLocalRoot(t, host, binding) })
	t.Run("planned-host-entries", func(t *testing.T) { testM3PlannedHostEntries(t, host, binding) })
	t.Run("protocol-selection", func(t *testing.T) { testM3ProtocolSelection(t, host, binding) })
	t.Run("business-operation-journal", func(t *testing.T) { testM3BusinessOperationJournal(t, host, binding) })
	t.Run("journal-host-entries", func(t *testing.T) {
		testM3JournalHostEntries(t, host, binding)
	})
	t.Run("operation-http-original-root", func(t *testing.T) {
		testM3OperationHTTPNative(t, host, binding)
	})
	t.Run("role-details-read", func(t *testing.T) {
		testRoleDetailsReadNative(t, host, binding)
	})
}

// This proves the host original-root executor against real MySQL. Half receipts
// are installed explicitly as a storage fixture; it is not Broker E2E evidence.
func testM3GuardedLocalRoot(t *testing.T, host *gorm.DB, binding transaction.Binding) {
	t.Helper()
	local, err := hostuow.NewTransactionUnitOfWork(host, nil, binding)
	if err != nil {
		t.Fatal(err)
	}
	if err = host.Exec("CREATE TABLE iam_m3_local_facts (operation_id VARBINARY(128) PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name                 string
		prepare, event, fail bool
		commit               bool
	}{
		{"all-halves-and-exact-event", true, true, false, true},
		{"frozen-role-id-and-audit", true, true, false, true},
		{"unprepared-half-never-starts-business", false, true, false, false},
		{"missing-event-rolls-back-facts", true, false, false, false},
		{"business-error-rolls-back-witness", true, true, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			current, err := policyrepo.NewPolicyVersionRepository(host).GetCurrent(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			id := "local-" + test.name
			command, err := change.NewCommandIntent("role.update", "user:7", "global", id, map[string]string{"id": "42"})
			if err != nil {
				t.Fatal(err)
			}
			at := time.Date(2026, 10, 19, 12, 1, 0, 0, time.UTC)
			plan, err := change.FreezeVersionPlan(command, current.Version, id, "event-"+id, at, "iam.authz.version.v2", "m3.policy.test")
			if err != nil {
				t.Fatal(err)
			}
			var artifacts change.LocalArtifacts
			if test.name == "frozen-role-id-and-audit" {
				builder := change.NewArtifactBuilder(at.Add(-time.Minute))
				if _, err = builder.ID(change.ArtifactKey("role", id)); err != nil {
					t.Fatal(err)
				}
				artifacts, err = builder.Freeze()
				if err != nil {
					t.Fatal(err)
				}
				plan, err = change.FreezeVersionPlanWithArtifacts(command, current.Version, id, "event-"+id, at, "iam.authz.version.v2", "m3.policy.test", artifacts)
				if err != nil {
					t.Fatal(err)
				}
			}
			reserved, err := local.Store().Reserve(t.Context(), transaction.RequestFor(plan), plan, transaction.ReserveOptions{Lease: time.Minute})
			if err != nil || !reserved.Acquired {
				t.Fatal(err)
			}
			e := reserved.Execution
			if err = local.Store().BeginPrepare(t.Context(), e); err != nil {
				t.Fatal(err)
			}
			if test.prepare {
				if err = local.Store().RecordHalf(t.Context(), e, e.References()[0], transaction.HalfReceipt{Outcome: transaction.HalfPrepared, BrokerMessageID: "fixture-message", BrokerTransactionID: "fixture-transaction"}); err != nil {
					t.Fatal(err)
				}
			}
			called := false
			executor := local.LocalExecutor(command, func(ctx context.Context, repos appuow.TxRepositories) error {
				called = true
				tx, err := dbmysql.RequireTx(ctx)
				if err != nil {
					return err
				}
				if err = tx.WithContext(ctx).Exec("INSERT INTO iam_m3_local_facts VALUES(?)", id).Error; err != nil {
					return err
				}
				if test.name == "frozen-role-id-and-audit" {
					if err = tx.Create(&rolepo.RolePO{Name: id, DisplayName: "frozen"}).Error; err != nil {
						return err
					}
				}
				version, err := repos.PolicyVersions.Increment(ctx, "system", "original guarded root")
				if err != nil {
					return err
				}
				if test.event {
					if err = change.StagePolicyVersionChanged(ctx, repos.Events, version); err != nil {
						return err
					}
				}
				if test.fail {
					return errors.New("explicit business rejection before commit")
				}
				return nil
			})
			outcome, localErr := executor(t.Context(), e)
			if test.commit {
				if localErr != nil || outcome != transaction.LocalCommitted {
					t.Fatal(outcome, localErr)
				}
			} else if localErr == nil || outcome != transaction.LocalAborted {
				t.Fatal("uncommitted root misclassified", outcome, localErr)
			}
			if called != test.prepare {
				t.Fatal("business gate did not follow complete half evidence")
			}
			checker := transaction.StoreChecker{Store: local.Store()}
			want := transaction.Unknown
			if test.commit {
				want = transaction.Commit
			}
			if checker.Check(t.Context(), e.References()[0]) != want {
				t.Fatal("original facts and exact witness diverged")
			}
			var count int64
			if err = host.Table("iam_m3_local_facts").Where("operation_id=?", id).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			expectedCount := int64(0)
			if test.commit {
				expectedCount = 1
			}
			if count != expectedCount {
				t.Fatal("partial local business fact", count)
			}
			after, err := policyrepo.NewPolicyVersionRepository(host).GetCurrent(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			expectedVersion := current.Version
			if test.commit {
				expectedVersion++
			}
			if after.Version != expectedVersion {
				t.Fatal("version diverged", after.Version)
			}
			if test.name == "frozen-role-id-and-audit" {
				var row rolepo.RolePO
				if err = host.Where("name=?", id).Take(&row).Error; err != nil {
					t.Fatal(err)
				}
				if row.ID != artifacts.IDs[change.ArtifactKey("role", id)] || !row.CreatedAt.Equal(artifacts.AuditAt) || !row.UpdatedAt.Equal(artifacts.AuditAt) {
					t.Fatal("ORM replaced frozen identity or audit clock")
				}
			}
			if !test.commit {
				aborted, err := local.Store().Abort(t.Context(), e)
				if err != nil || !aborted {
					t.Fatal("no persisted abort fence", err)
				}
				if checker.Check(t.Context(), e.References()[0]) != transaction.Rollback {
					t.Fatal("abort fence not authoritative")
				}
			}
			if _, err = executor(t.Context(), e); !errors.Is(err, transaction.ErrStaleExecution) {
				t.Fatal("local executor repeated business", err)
			}
		})
	}
	if err = dbmysql.NewUnitOfWork(host).WithinTransaction(t.Context(), func(ctx context.Context) error {
		tx, err := dbmysql.RequireTx(ctx)
		if err != nil {
			return err
		}
		if _, err = hostuow.NewTransactionUnitOfWork(tx, nil, binding); !errors.Is(err, transaction.ErrTransactionRequired) {
			return errors.New("constructor borrowed an existing root")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
