//go:build integration

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sessioncache "github.com/FangcunMount/iam/v5/internal/apiserver/infra/cache/redis"
	"github.com/FangcunMount/iam/v5/pkg/version"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// IAM actions here are real isolated CLI processes. Every QS receipt, execution
// and clearance below is an explicit synthetic interface fixture. This lane
// does not run QS, observe a QS database, or qualify a real global clearance.
func cliNativeClearanceCycle(t *testing.T, ctx context.Context, conn *sql.Conn, rc *redis.Client, prior *cliNativeFixture, old, retained iamScope, catalogRaw []byte) {
	t.Helper()
	dir := filepath.Join(prior.dir, "clearance-v2")
	require.NoError(t, os.Mkdir(dir, 0700))
	f := &cliNativeFixture{ctx: ctx, dir: dir, dsn: prior.dsn, redisURL: prior.redisURL, files: map[string]inputFile{}}
	scope := cliNativeSeed(t, ctx, conn, rc, 830000301)
	keep := iamScope{appendUnique(retained.Users, old.Users...), appendUnique(retained.Profiles, old.Profiles...), appendUnique(retained.LoginIdentities, old.LoginIdentities...)}
	protectedRaw := encode(protection{1, "mock_cohort_protected", keep, json.RawMessage(`{"system":[],"clack":[],"other":[]}`)})
	binding := qsDatabaseBinding{"synthetic-qs-server", "synthetic-qs-sql", strings.Repeat("c", 64), "synthetic-qs-mongo", 101, 39}
	qs := mustMarshal(map[string]any{"binding": binding, "selector": map[string]string{"source": "native-v2-synthetic-qs-interface"}})
	ownerRaw := encode(ownerConfirmation{1, "mock_cohort_owner_confirmation", "native-cli-clearance-v2", "10001", jsonSHA(scope), rawSHA(qs), rawSHA(protectedRaw), true, true, []string{"1"}})
	g := groupManifest{1, "mock_cohort", "native-cli-clearance-v2", rawSHA(ownerRaw), rawSHA(protectedRaw), mustMarshal(scope), qs}
	f.put(t, "group", encode(g))
	f.put(t, "owner-confirmation", ownerRaw)
	f.put(t, "protected", protectedRaw)
	f.put(t, "event-catalog", catalogRaw)
	f.files["archive"] = inputFile{path: filepath.Join(dir, "archive.json")}
	captured := f.success(t, "capture", f.args("capture"))
	f.files["archive"] = inputFile{f.files["archive"].path, captured.ArchiveSHA}
	archiveRaw, err := load(f.files["archive"])
	require.NoError(t, err)
	var a archive
	require.NoError(t, strictJSON(archiveRaw, &a))
	f.put(t, "frozen", encode(freezeReceipt{1, "mock_cohort_frozen", g.Operation, f.files["group"].sha, true, true, true, true}))
	f.files["messages-settled"] = f.files["frozen"]
	materialSHA := strings.Repeat("d", 64)
	for _, action := range []string{"capture", "apply"} {
		f.put(t, "qs-"+action, encode(qsActionReceipt{1, "mock_cohort_qs_action_result", action, g.Operation, f.files["group"].sha, strings.Repeat("b", 40), materialSHA, true, false, false, action == "apply", 0, true}))
	}
	batch := qsClearanceBatch{GroupSHA: f.files["group"].sha, OwnerSHA: g.OwnerSHA, IAMSHA: rawSHA(g.IAM), QSSHA: rawSHA(g.QS), IAMCounts: scope.counts(), MaterialSHA: materialSHA, CaptureSHA: f.files["qs-capture"].sha, CaptureEvidenceSHA: strings.Repeat("e", 64), PartialApplySHA: f.files["qs-apply"].sha, PartialApplyExecutionSHA: strings.Repeat("f", 64), JournalSHA: strings.Repeat("a", 64), CurrentFrozenSHA: f.files["frozen"].sha}
	now := time.Now().UTC()
	clearance := qsClearanceReceipt{Format: 1, Kind: "mock_cohort_qs_clearance", Action: "clearance", Operation: g.Operation, Source: strings.Repeat("a", 40), BatchSHA: strings.Repeat("a", 64), ProtectedSHA: g.ProtectedSHA, Binding: binding, Started: now.Add(-time.Second), Completed: now, Expires: now.Add(3 * time.Minute), Batches: []qsClearanceBatch{batch}, Absent: true, Protected: true, ReferencesClear: true, Complete: true, Released: true}
	f.put(t, "qs-clearance", encode(clearance))
	bind := func(action, priorSHA string, exit int) {
		f.put(t, "qs-clearance-execution", encode(qsClearanceExecution{1, "mock_cohort_clearance_execution", f.files["qs-clearance"].path, f.files["qs-clearance"].sha, "exited", exit}))
		h := qsBatchHandoff{Format: 1, Kind: "mock_cohort_qs_batch_cleared", Operation: g.Operation, GroupSHA: batch.GroupSHA, OwnerSHA: g.OwnerSHA, ProtectedSHA: g.ProtectedSHA, IAMSHA: rawSHA(g.IAM), QSSHA: rawSHA(g.QS), IAMCounts: scope.counts(), BatchManifestSHA: clearance.BatchSHA, CaptureSHA: batch.CaptureSHA, PartialApplySHA: batch.PartialApplySHA, PartialApplyExecutionSHA: batch.PartialApplyExecutionSHA, ClearanceSHA: f.files["qs-clearance"].sha, ClearanceExecutionSHA: f.files["qs-clearance-execution"].sha, CurrentFrozenSHA: f.files["frozen"].sha}
		f.put(t, "qs-handoff", encode(h))
		issued := time.Now().UTC()
		f.put(t, "approval", encode(approval{1, "mock_cohort_iam_approval", action, g.Operation, version.GitCommit, f.files["group"].sha, f.files["archive"].sha, f.files["qs-handoff"].sha, f.files["frozen"].sha, f.files["event-catalog"].sha, priorSHA, a.Identity, issued, issued.Add(time.Minute), clearance.Expires}))
	}
	args := func(action string) []string {
		out := f.args(action)
		if action == "apply" || action == "restore" {
			for _, name := range []string{"qs-clearance", "qs-clearance-execution"} {
				in := f.files[name]
				out = append(out, "--"+name+"-file="+in.path, "--"+name+"-sha256="+in.sha)
			}
		}
		return out
	}
	f.files["journal"] = inputFile{path: filepath.Join(dir, "apply.jsonl")}
	bind("apply", "", 1)
	failed, code := f.invoke(t, args("apply"))
	require.Equal(t, 1, code)
	require.False(t, failed.Complete || failed.SQLClosed || failed.RedisClosed || failed.GateReleased, "failed synthetic QS process proof must reject before acquiring IAM resources")
	_, err = os.Stat(f.files["archive"].path + ".apply-claim")
	require.True(t, os.IsNotExist(err))
	bind("apply", "", 0)
	applied := f.success(t, "apply", args("apply"))
	journalRaw, err := readPrivate(f.files["journal"].path, applied.JournalSHA, maxDocument)
	require.NoError(t, err)
	entries, err := parseJournal(journalRaw, g.Operation, f.files["archive"].sha)
	require.NoError(t, err)
	require.Equal(t, "complete", entries[len(entries)-1].Result)
	f.success(t, "verify", args("verify"))
	f.files["prior-journal"] = inputFile{f.files["journal"].path, applied.JournalSHA}
	f.files["journal"] = inputFile{path: filepath.Join(dir, "restore.jsonl")}
	bind("restore", applied.JournalSHA, 0)
	f.success(t, "restore", args("restore"))
	got, err := readScope(ctx, conn, scope)
	require.NoError(t, err)
	require.True(t, imageRowsEqual(a.Tables, got))
	protected, err := readScope(ctx, conn, iamScope{appendUnique(keep.Users, "10001", "618855329043591726"), appendUnique(keep.Profiles, "632367179001508398", "636374014138266158"), keep.LoginIdentities})
	require.NoError(t, err)
	require.True(t, imageRowsEqual(a.Protected, protected))
	stillOriginal, err := load(f.files["prior-journal"])
	require.NoError(t, err)
	require.Equal(t, journalRaw, stillOriginal)
	for _, sid := range []string{"indexed-830000301", "unindexed-830000301"} {
		s, er := sessioncache.NewSessionStore(rc).Get(ctx, sid)
		require.NoError(t, er)
		require.NotNil(t, s)
		require.False(t, s.IsActive())
	}
	t.Log("real IAM v2 capture/apply/verify/restore subprocesses passed with immutable partial QS originals; QS clearance and execution remain synthetic interface fixtures")
}
