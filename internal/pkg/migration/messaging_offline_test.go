package migration

import (
	"context"
	"database/sql"
	"testing"
)

func TestOfflineMessagingRequiresDeadlineBeforeAnyConnection(t *testing.T) {
	db := new(sql.DB) // Any attempted connection on this uninitialized pool would fail.
	for _, ctx := range []context.Context{nil, context.Background()} {
		if _, err := RunMessagingOffline(ctx, db, 41); err == nil {
			t.Fatal("unbounded migration accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RunMessagingOffline(ctx, db, 41); err == nil {
		t.Fatal("canceled migration accepted")
	}
}

func TestProduction40UsesOriginalAuditMigration(t *testing.T) {
	name := messagingMigrationFile(41)
	if name != "000041_message_failure_audit.up.sql" {
		t.Fatal("production40 bridge omitted the original additive audit migration")
	}
	if data, err := migrations.ReadFile("migrations/" + name); err != nil || len(data) == 0 {
		t.Fatal("original audit migration unavailable", err)
	}
}
