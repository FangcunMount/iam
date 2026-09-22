package eventoutbox

import (
	"testing"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	"github.com/stretchr/testify/require"
)

func TestPolicyIntentPreservesHistoricalIdentityAndBytes(t *testing.T) {
	row := OutboxPO{EventID: "historical-1", EventType: eventing.AuthzVersionChanged, AggregateType: "PolicyVersion", AggregateID: "2", TopicName: "iam.authz.version.v2", PayloadJSON: "{ \"version\":2, \"extension\":{\"keep\":true} }", CreatedAt: time.Date(2026, 9, 22, 10, 0, 0, 123000000, time.FixedZone("CST", 8*3600))}
	first, err := policyIntent(row, row.TopicName)
	require.NoError(t, err)
	in := first.Input()
	require.Equal(t, row.EventID, in.ID)
	require.Equal(t, []byte(row.PayloadJSON), in.Payload)
	require.Equal(t, "scope:global", in.Scope)
	require.Equal(t, "2026-09-22T02:00:00.123Z", in.OccurredAt)
	row.Status = "failed"
	row.AttemptCount = 7
	row.UpdatedAt = time.Now()
	retried, err := policyIntent(row, row.TopicName)
	require.NoError(t, err)
	require.Equal(t, first.Fingerprint(), retried.Fingerprint())
	for _, tc := range []struct {
		name   string
		mutate func(*OutboxPO)
	}{
		{"unknown event", func(r *OutboxPO) { r.EventType = "unknown" }},
		{"unknown aggregate", func(r *OutboxPO) { r.AggregateType = "TenantPolicy" }},
		{"wrong route", func(r *OutboxPO) { r.TopicName = "other" }},
		{"missing ID", func(r *OutboxPO) { r.EventID = "" }},
		{"missing time", func(r *OutboxPO) { r.CreatedAt = time.Time{} }},
		{"version mismatch", func(r *OutboxPO) { r.AggregateID = "3" }},
		{"invalid JSON", func(r *OutboxPO) { r.PayloadJSON = "broken" }},
		{"nonpositive version", func(r *OutboxPO) { r.PayloadJSON = `{"version":0}` }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := row
			tc.mutate(&changed)
			_, err := policyIntent(changed, "iam.authz.version.v2")
			require.Error(t, err)
		})
	}
}
