package scopemigrate

import (
	"context"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"os"
	"testing"
	"time"
)

func TestConcurrentScopeApplySerializesAtPolicyMySQL(t *testing.T) {
	if os.Getenv("SCOPE_MIGRATION_MYSQL_DSN") == "" {
		if os.Getenv("SCOPE_MIGRATION_REQUIRE_MYSQL") == "true" {
			t.Fatal("SCOPE_MIGRATION_MYSQL_DSN required")
		}
		t.Skip("isolated MySQL required for real row lock verification")
	}
	iam, qs, input := applyFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := iam.AutoMigrate(&eventoutbox.OutboxPO{}); err != nil {
		t.Fatal(err)
	}
	cfg, err := eventcatalog.Load("../../../../configs/events.yaml")
	if err != nil {
		t.Fatal(err)
	}
	store := eventoutbox.NewStore(iam, eventcatalog.NewCatalog(cfg))
	before, err := LoadSnapshot(ctx, iam, qs)
	if err != nil {
		t.Fatal(err)
	}
	var schema string
	if err := iam.Raw("SELECT DATABASE()").Scan(&schema).Error; err != nil {
		t.Fatal(err)
	}
	blocker := iam.WithContext(ctx).Begin()
	if blocker.Error != nil {
		t.Fatal(blocker.Error)
	}
	defer blocker.Rollback()
	var version int64
	if err := blocker.Raw("SELECT policy_version FROM authz_policy_versions WHERE id=1 FOR UPDATE").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		receipt *Receipt
		err     error
	}
	results := make(chan outcome, 2)
	for i := 0; i < 2; i++ {
		go func() {
			receipt, e := Apply(ctx, iam, qs, store, "concurrent", "9", before.Hash(), input, true)
			results <- outcome{receipt, e}
		}()
	}
	// Observe both actual InnoDB lock waits before releasing the policy row.
	deadline := time.Now().Add(5 * time.Second)
	waiting := false
	for time.Now().Before(deadline) {
		var n int64
		query := `SELECT COUNT(DISTINCT w.REQUESTING_ENGINE_TRANSACTION_ID) FROM performance_schema.data_lock_waits w JOIN performance_schema.data_locks l ON l.ENGINE_LOCK_ID=w.REQUESTING_ENGINE_LOCK_ID WHERE l.OBJECT_SCHEMA=? AND l.OBJECT_NAME='authz_policy_versions'`
		if err := iam.WithContext(ctx).Raw(query, schema).Scan(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n >= 2 {
			waiting = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !waiting {
		t.Fatal("did not observe both concurrent requests waiting on policy row")
	}
	if err := blocker.Commit().Error; err != nil {
		t.Fatal(err)
	}
	var hash string
	for i := 0; i < 2; i++ {
		select {
		case result := <-results:
			if result.err != nil || result.receipt == nil {
				t.Fatalf("concurrent apply failed: %v", result.err)
			}
			if hash != "" && hash != result.receipt.AfterHash {
				t.Fatal("concurrent requests produced distinct receipts")
			}
			hash = result.receipt.AfterHash
		case <-ctx.Done():
			t.Fatal("concurrent apply timed out")
		}
	}
	var rows int64
	if err := iam.Model(&eventoutbox.OutboxPO{}).Count(&rows).Error; err != nil || rows != 1 {
		t.Fatalf("duplicate durable events %d %v", rows, err)
	}
	after, err := LoadSnapshot(ctx, iam, qs)
	if err != nil || after.IAM.PolicyVersion != before.IAM.PolicyVersion+1 {
		t.Fatal("policy advanced more than once")
	}
}
