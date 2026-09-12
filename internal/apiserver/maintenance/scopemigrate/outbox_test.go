package scopemigrate

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"testing"
)

type failAfterDurableStage struct{ store event.Stager }

func (s failAfterDurableStage) Stage(ctx context.Context, events ...event.DomainEvent) error {
	if err := s.store.Stage(ctx, events...); err != nil {
		return err
	}
	return fmt.Errorf("injected failure after durable insert")
}
func TestScopeRealOutboxCommitsAndRollsBackWithPolicy(t *testing.T) {
	iam, qs, input := applyFixture(t)
	ctx := context.Background()
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
	// Failure after the real Store inserts the event must roll back its row too.
	if _, err := Apply(ctx, iam, qs, failAfterDurableStage{store}, "durable", "9", before.Hash(), input, true); err == nil {
		t.Fatal("injected failure accepted")
	}
	unchanged, err := LoadSnapshot(ctx, iam, qs)
	if err != nil || unchanged.Hash() != before.Hash() {
		t.Fatal("failed durable apply altered policy")
	}
	var n int64
	if err := iam.Model(&eventoutbox.OutboxPO{}).Count(&n).Error; err != nil || n != 0 {
		t.Fatal("failed transaction retained real outbox event")
	}
	for i := 0; i < 2; i++ {
		if _, err := Apply(ctx, iam, qs, store, "durable", "9", before.Hash(), input, true); err != nil {
			t.Fatal(err)
		}
	}
	applied, err := LoadSnapshot(ctx, iam, qs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Rollback(ctx, iam, qs, failAfterDurableStage{store}, "durable", "9", before.Hash(), true); err == nil {
		t.Fatal("failed rollback accepted")
	}
	current, err := LoadSnapshot(ctx, iam, qs)
	if err != nil || current.Hash() != applied.Hash() {
		t.Fatal("failed rollback changed policy")
	}
	for i := 0; i < 2; i++ {
		if _, err := Rollback(ctx, iam, qs, store, "durable", "9", before.Hash(), true); err != nil {
			t.Fatal(err)
		}
	}
	var rows []eventoutbox.OutboxPO
	if err := iam.Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected exactly apply and rollback events, got %d", len(rows))
	}
	for i, row := range rows {
		var payload struct {
			Version int64 `json:"version"`
		}
		if err := json.Unmarshal([]byte(row.PayloadJSON), &payload); err != nil {
			t.Fatal(err)
		}
		expected := int64(6 + i)
		if row.EventID == "" || row.EventType != "iam.authz.version_changed.v2" || row.TopicName != "iam.authz.version.v2" || row.AggregateType != "PolicyVersion" || row.AggregateID != fmt.Sprint(expected) || row.Status != "pending" || payload.Version != expected {
			t.Fatalf("invalid runtime policy event %+v", row)
		}
	}
	if rows[0].EventID == rows[1].EventID {
		t.Fatal("event identity reused")
	}
}
