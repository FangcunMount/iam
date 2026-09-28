package outbox_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	iamoutbox "github.com/FangcunMount/iam/v5/pkg/outbox"
	rmoutbox "github.com/FangcunMount/reliable-messaging/outbox"
)

func TestPublicStatusIdentityAndSDKConversion(t *testing.T) {
	const iamPath = "github.com/FangcunMount/iam/v5/pkg/outbox"
	if got := reflect.TypeOf(iamoutbox.StatusBucket{}).PkgPath(); got != iamPath {
		t.Fatalf("StatusBucket Go identity moved to %q", got)
	}
	if got := reflect.TypeOf(iamoutbox.StatusSnapshot{}).PkgPath(); got != iamPath {
		t.Fatalf("StatusSnapshot Go identity moved to %q", got)
	}
	if got := reflect.TypeOf((*iamoutbox.StatusReader)(nil)).Elem().PkgPath(); got != iamPath {
		t.Fatalf("StatusReader Go identity moved to %q", got)
	}

	generated := time.Date(2026, 9, 28, 10, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	oldest := generated.Add(-2 * time.Minute)
	public := iamoutbox.StatusSnapshot{
		Store: "iam-standard-and-legacy-outbox", GeneratedAt: generated,
		Buckets: []iamoutbox.StatusBucket{{Status: "standard_retry_wait", Count: 2, OldestCreatedAt: &oldest, OldestAgeSeconds: 120}},
	}
	shared := public.ToSDK()
	var sdk rmoutbox.StatusSnapshot = shared
	if sdk.Store != public.Store || !sdk.GeneratedAt.Equal(public.GeneratedAt) || len(sdk.Buckets) != 1 {
		t.Fatalf("SDK status conversion lost snapshot data: %+v", sdk)
	}
	if sdk.Buckets[0].Status != public.Buckets[0].Status || sdk.Buckets[0].Count != public.Buckets[0].Count ||
		!sdk.Buckets[0].OldestCreatedAt.Equal(*public.Buckets[0].OldestCreatedAt) ||
		sdk.Buckets[0].OldestAgeSeconds != public.Buckets[0].OldestAgeSeconds {
		t.Fatalf("SDK status conversion lost bucket data: %+v", sdk.Buckets[0])
	}
	publicJSON, err := json.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	sharedJSON, err := json.Marshal(shared)
	if err != nil {
		t.Fatal(err)
	}
	if string(publicJSON) != string(sharedJSON) {
		t.Fatalf("status JSON changed: public=%s SDK=%s", publicJSON, sharedJSON)
	}
}
