package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func clearanceContractFixture(t *testing.T) (groupManifest, iamScope, qsBatchHandoff, qsClearanceReceipt, time.Time) {
	t.Helper()
	_, _, _, g, s := groupFixture(t)
	sha := strings.Repeat("a", 64)
	binding := qsDatabaseBinding{"qs-uuid", "qs", sha, "qs-mongo", 101, 39}
	g.QS = mustMarshal(map[string]any{"binding": binding, "selector": map[string]string{"source": "synthetic-clearance-contract"}})
	groupSHA := rawSHA(encode(g))
	h := qsBatchHandoff{Format: 1, Kind: "mock_cohort_qs_batch_cleared", Operation: g.Operation, GroupSHA: groupSHA, OwnerSHA: g.OwnerSHA, ProtectedSHA: g.ProtectedSHA, IAMSHA: rawSHA(g.IAM), QSSHA: rawSHA(g.QS), IAMCounts: s.counts(), BatchManifestSHA: sha, CaptureSHA: sha, PartialApplySHA: sha, PartialApplyExecutionSHA: sha, ClearanceSHA: sha, ClearanceExecutionSHA: sha, CurrentFrozenSHA: sha}
	b := qsClearanceBatch{GroupSHA: groupSHA, OwnerSHA: g.OwnerSHA, IAMSHA: rawSHA(g.IAM), QSSHA: rawSHA(g.QS), IAMCounts: s.counts(), MaterialSHA: sha, CaptureSHA: sha, CaptureEvidenceSHA: sha, PartialApplySHA: sha, PartialApplyExecutionSHA: sha, JournalSHA: sha, CurrentFrozenSHA: sha}
	now := time.Now().UTC()
	c := qsClearanceReceipt{Format: 1, Kind: "mock_cohort_qs_clearance", Action: "clearance", Operation: g.Operation, Source: strings.Repeat("b", 40), BatchSHA: sha, ProtectedSHA: g.ProtectedSHA, Binding: binding, Started: now.Add(-time.Minute), Completed: now.Add(-time.Second), Expires: now.Add(4 * time.Minute), Batches: []qsClearanceBatch{b}, Absent: true, Protected: true, ReferencesClear: true, Complete: true, Released: true}
	return g, s, h, c, now
}

func TestQSClearanceRequiresExplicitCompleteOptIn(t *testing.T) {
	sha := strings.Repeat("a", 64)
	flags := []string{"--qs-clearance-file=/private/clearance", "--qs-clearance-sha256=" + sha, "--qs-clearance-execution-file=/private/execution", "--qs-clearance-execution-sha256=" + sha}
	for mask := 0; mask < 16; mask++ {
		args := []string{"--action=apply"}
		for i, flag := range flags {
			if mask&(1<<i) != 0 {
				args = append(args, flag)
			}
		}
		o, err := parseArgs(args)
		if mask != 0 && mask != 15 {
			if err == nil {
				t.Fatalf("incomplete opt-in admitted: %d", mask)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		mode, err := qsClearanceMode(o)
		if err != nil || mode != (mask == 15) {
			t.Fatal("old or explicit branch changed", err)
		}
	}
	for _, action := range []string{"plan", "capture", "verify"} {
		if _, err := parseArgs(append([]string{"--action=" + action}, flags...)); err == nil {
			t.Fatal("read-only action silently ignored clearance flags")
		}
	}
	if _, err := parseArgs(append(append([]string{"--action=restore"}, flags...), "--qs-verify-file=/private/old")); err == nil {
		t.Fatal("mixed old verify and current clearance admitted")
	}
}

func TestQSClearanceExplicitEmptyFlagsCannotFallBack(t *testing.T) {
	names := []string{"qs-clearance-file", "qs-clearance-sha256", "qs-clearance-execution-file", "qs-clearance-execution-sha256"}
	for _, action := range []string{"plan", "capture", "apply", "verify", "restore"} {
		for mask := 1; mask < 16; mask++ {
			args := []string{"--action=" + action}
			for i, name := range names {
				if mask&(1<<i) != 0 {
					args = append(args, "--"+name+"=")
				}
			}
			if _, err := parseArgs(args); err == nil {
				t.Fatalf("explicit empty clearance flags fell back to historical mode: %v", args)
			}
		}
	}
	if _, err := parseArgs([]string{"--action=apply", "--qs-clearance-file", ""}); err == nil {
		t.Fatal("separate explicit empty value fell back to historical mode")
	}
}

func TestQSBatchHandoffIsSeparateAndBindsExactTypedScope(t *testing.T) {
	g, s, h, _, _ := clearanceContractFixture(t)
	if _, err := validateQSBatchHandoff(encode(h), g, s, h.GroupSHA); err != nil {
		t.Fatal(err)
	}
	if _, err := validateHandoff(encode(h), g, s, h.GroupSHA); err == nil {
		t.Fatal("historical handoff accepted a v2 document")
	}
	for _, edit := range []func(*qsBatchHandoff){
		func(h *qsBatchHandoff) { h.Kind = "mock_cohort_qs_cleared" },
		func(h *qsBatchHandoff) { h.GroupSHA = strings.Repeat("b", 64) },
		func(h *qsBatchHandoff) { h.OwnerSHA = strings.Repeat("b", 64) },
		func(h *qsBatchHandoff) { h.ProtectedSHA = strings.Repeat("b", 64) },
		func(h *qsBatchHandoff) { h.IAMSHA = strings.Repeat("b", 64) },
		func(h *qsBatchHandoff) { h.QSSHA = strings.Repeat("b", 64) },
		func(h *qsBatchHandoff) { h.IAMCounts.Users++ },
		func(h *qsBatchHandoff) { h.PartialApplyExecutionSHA = "" },
		func(h *qsBatchHandoff) { h.ClearanceExecutionSHA = "" },
		func(h *qsBatchHandoff) { h.CurrentFrozenSHA = "" },
	} {
		bad := h
		edit(&bad)
		if _, err := validateQSBatchHandoff(encode(bad), g, s, h.GroupSHA); err == nil {
			t.Fatal("unbound v2 handoff admitted")
		}
	}
}

func TestQSClearanceBindsCurrentExactBatchAndCleanDatabase(t *testing.T) {
	g, s, h, c, now := clearanceContractFixture(t)
	if _, member, err := validateQSClearance(encode(c), g, s, h.GroupSHA, h, now); err != nil || member != c.Batches[0] {
		t.Fatal("current exact batch rejected", err)
	}
	for _, tc := range []struct {
		name string
		edit func(*qsClearanceReceipt)
	}{
		{"operation", func(c *qsClearanceReceipt) { c.Operation = "other" }},
		{"batch-manifest", func(c *qsClearanceReceipt) { c.BatchSHA = strings.Repeat("c", 64) }},
		{"protected-manifest", func(c *qsClearanceReceipt) { c.ProtectedSHA = strings.Repeat("c", 64) }},
		{"database", func(c *qsClearanceReceipt) { c.Binding.SQLDatabase = "other" }},
		{"schema", func(c *qsClearanceReceipt) { c.Binding.MongoVersion++ }},
		{"source", func(c *qsClearanceReceipt) { c.Source = "unknown" }},
		{"absent", func(c *qsClearanceReceipt) { c.Absent = false }},
		{"protected", func(c *qsClearanceReceipt) { c.Protected = false }},
		{"references", func(c *qsClearanceReceipt) { c.ReferencesClear = false }},
		{"complete", func(c *qsClearanceReceipt) { c.Complete = false }},
		{"release", func(c *qsClearanceReceipt) { c.Released = false }},
		{"missing-member", func(c *qsClearanceReceipt) { c.Batches = []qsClearanceBatch{} }},
		{"wrong-member", func(c *qsClearanceReceipt) { c.Batches[0].GroupSHA = strings.Repeat("c", 64) }},
		{"duplicate-member", func(c *qsClearanceReceipt) { c.Batches = append(c.Batches, c.Batches[0]) }},
		{"raw-iam", func(c *qsClearanceReceipt) { c.Batches[0].IAMSHA = strings.Repeat("c", 64) }},
		{"raw-qs", func(c *qsClearanceReceipt) { c.Batches[0].QSSHA = strings.Repeat("c", 64) }},
		{"owner", func(c *qsClearanceReceipt) { c.Batches[0].OwnerSHA = strings.Repeat("c", 64) }},
		{"typed-counts", func(c *qsClearanceReceipt) { c.Batches[0].IAMCounts.LoginIdentities++ }},
		{"apply-original", func(c *qsClearanceReceipt) { c.Batches[0].PartialApplySHA = strings.Repeat("c", 64) }},
		{"apply-exit", func(c *qsClearanceReceipt) { c.Batches[0].PartialApplyExecutionSHA = strings.Repeat("c", 64) }},
		{"freeze", func(c *qsClearanceReceipt) { c.Batches[0].CurrentFrozenSHA = strings.Repeat("c", 64) }},
		{"missing-journal", func(c *qsClearanceReceipt) { c.Batches[0].JournalSHA = "" }},
		{"missing-capture-proof", func(c *qsClearanceReceipt) { c.Batches[0].CaptureEvidenceSHA = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := c
			bad.Batches = append([]qsClearanceBatch{}, c.Batches...)
			tc.edit(&bad)
			if _, _, err := validateQSClearance(encode(bad), g, s, h.GroupSHA, h, now); err == nil {
				t.Fatal("unsafe clearance admitted")
			}
		})
	}
	for _, raw := range [][]byte{
		[]byte(strings.Replace(string(encode(c)), `"complete":true`, `"complete":null`, 1)),
		[]byte(strings.Replace(string(encode(c)), `"complete":true`, `"complete":true,"complete":true`, 1)),
		[]byte(strings.Replace(string(encode(c)), `"resource_release_proven":true`, `"Resource_release_proven":true`, 1)),
	} {
		if _, _, err := validateQSClearance(raw, g, s, h.GroupSHA, h, now); err == nil {
			t.Fatal("nonliteral clearance wire admitted")
		}
	}
}

func TestQSClearanceCannotExtendObservationOrIAMApproval(t *testing.T) {
	_, _, _, c, now := clearanceContractFixture(t)
	a := approval{Expires: now.Add(time.Minute), Deadline: c.Expires}
	if err := validateQSClearanceApproval(c, a, now); err != nil {
		t.Fatal(err)
	}
	for _, edit := range []func(*qsClearanceReceipt){
		func(c *qsClearanceReceipt) { c.Started = time.Time{} },
		func(c *qsClearanceReceipt) { c.Completed = c.Started.Add(-time.Nanosecond) },
		func(c *qsClearanceReceipt) { c.Completed = now.Add(time.Second) },
		func(c *qsClearanceReceipt) { c.Completed = c.Expires },
		func(c *qsClearanceReceipt) { c.Expires = now },
		func(c *qsClearanceReceipt) { c.Expires = c.Started.Add(25*time.Minute + time.Nanosecond) },
	} {
		bad := c
		edit(&bad)
		if validateQSClearanceApproval(bad, a, now) == nil {
			t.Fatal("invalid current observation window admitted")
		}
	}
	for _, app := range []approval{{Expires: c.Expires.Add(time.Nanosecond), Deadline: c.Expires}, {Expires: c.Expires, Deadline: c.Expires.Add(time.Nanosecond)}} {
		if validateQSClearanceApproval(c, app, now) == nil {
			t.Fatal("IAM approval extended current clearance")
		}
	}
	if validateQSClearanceApproval(c, a, c.Expires) == nil {
		t.Fatal("expired observation survived a later COMMIT boundary")
	}
}

func TestQSPartialApplyRequiresCompleteReleasedPendingBatch(t *testing.T) {
	_, _, h, c, _ := clearanceContractFixture(t)
	r := qsActionReceipt{Format: 1, Kind: "mock_cohort_qs_action_result", Action: "apply", Operation: h.Operation, GroupSHA: h.GroupSHA, Source: strings.Repeat("d", 40), MaterialSHA: c.Batches[0].MaterialSHA, Complete: true, SemanticPending: true, Released: true}
	if _, err := validateQSPartialApply(encode(r), h.Operation, h.GroupSHA); err != nil {
		t.Fatal(err)
	}
	if _, err := validateQSAction(encode(r), "apply", h.Operation, h.GroupSHA); err == nil {
		t.Fatal("historical whole-cohort branch accepted a partial apply")
	}
	for _, edit := range []func(*qsActionReceipt){
		func(r *qsActionReceipt) { r.Complete = false },
		func(r *qsActionReceipt) { r.DryRun = true },
		func(r *qsActionReceipt) { r.Whole = true },
		func(r *qsActionReceipt) { r.SemanticPending = false },
		func(r *qsActionReceipt) { r.Excluded = 1 },
		func(r *qsActionReceipt) { r.Released = false },
		func(r *qsActionReceipt) { r.Source = "" },
	} {
		bad := r
		edit(&bad)
		if _, err := validateQSPartialApply(encode(bad), h.Operation, h.GroupSHA); err == nil {
			t.Fatal("failed or relabelled partial action admitted")
		}
	}
}

type clearanceFileFixture struct {
	o       options
	g       groupManifest
	s       iamScope
	h       qsBatchHandoff
	c       qsClearanceReceipt
	proof   qsClearanceExecution
	app     approval
	now     time.Time
	dir     string
	seq     int
	owner   []byte
	protect []byte
}

func (f *clearanceFileFixture) put(t *testing.T, name string, raw []byte) inputFile {
	t.Helper()
	f.seq++
	p := filepath.Join(f.dir, name+strings.Repeat("x", f.seq)+".json")
	if err := writeExclusive(p, raw); err != nil {
		t.Fatal(err)
	}
	return inputFile{p, rawSHA(raw)}
}

func (f *clearanceFileFixture) bind(t *testing.T) {
	t.Helper()
	f.o.qsClearance = f.put(t, "clearance", encode(f.c))
	f.proof.ReceiptFile, f.proof.ReceiptSHA = f.o.qsClearance.path, f.o.qsClearance.sha
	f.o.qsClearanceExecution = f.put(t, "clearance-execution", encode(f.proof))
	f.h.ClearanceSHA, f.h.ClearanceExecutionSHA = f.o.qsClearance.sha, f.o.qsClearanceExecution.sha
	f.o.handoff = f.put(t, "handoff", encode(f.h))
	f.app.HandoffSHA = f.o.handoff.sha
	f.o.approval = f.put(t, "approval", encode(f.app))
}

func newClearanceFileFixture(t *testing.T) *clearanceFileFixture {
	t.Helper()
	g, s, h, c, now := clearanceContractFixture(t)
	_, ownerRaw, protectedRaw, _, _ := groupFixture(t)
	var owner ownerConfirmation
	if err := strictJSON(ownerRaw, &owner); err != nil {
		t.Fatal(err)
	}
	owner.QSSHA = rawSHA(g.QS)
	ownerRaw = encode(owner)
	g.OwnerSHA = rawSHA(ownerRaw)
	groupSHA := rawSHA(encode(g))
	h.GroupSHA, h.OwnerSHA = groupSHA, g.OwnerSHA
	c.Batches[0].GroupSHA, c.Batches[0].OwnerSHA = groupSHA, g.OwnerSHA
	f := &clearanceFileFixture{g: g, s: s, h: h, c: c, now: now, dir: t.TempDir(), owner: ownerRaw, protect: protectedRaw}
	if err := os.Chmod(f.dir, 0700); err != nil {
		t.Fatal(err)
	}
	f.o.action = "apply"
	f.o.deadline = time.Minute
	f.o.group = f.put(t, "group", encode(g))
	f.o.owner = f.put(t, "owner", ownerRaw)
	f.o.protected = f.put(t, "protected", protectedRaw)
	f.o.frozen = f.put(t, "freeze", encode(freezeReceipt{1, "mock_cohort_frozen", g.Operation, groupSHA, true, true, true, true}))
	f.o.messages = f.o.frozen
	f.h.CurrentFrozenSHA, f.c.Batches[0].CurrentFrozenSHA = f.o.frozen.sha, f.o.frozen.sha
	r := qsActionReceipt{Format: 1, Kind: "mock_cohort_qs_action_result", Action: "capture", Operation: g.Operation, GroupSHA: groupSHA, Source: strings.Repeat("d", 40), MaterialSHA: c.Batches[0].MaterialSHA, Complete: true, Released: true}
	f.o.qsCapture = f.put(t, "capture", encode(r))
	r.Action, r.SemanticPending = "apply", true
	f.o.qsApply = f.put(t, "apply", encode(r))
	f.h.CaptureSHA, f.c.Batches[0].CaptureSHA = f.o.qsCapture.sha, f.o.qsCapture.sha
	f.h.PartialApplySHA, f.c.Batches[0].PartialApplySHA = f.o.qsApply.sha, f.o.qsApply.sha
	id := identity{Database: "iam", ServerUUID: "iam-uuid", Version: 40, RedisSHA: strings.Repeat("c", 64)}
	a := archive{Format: 1, Kind: "mock_cohort_iam_archive", Operation: g.Operation, Source: strings.Repeat("d", 40), GroupSHA: groupSHA, Identity: id, Tables: []tableImage{}, Protected: []tableImage{}}
	for _, name := range tableOrder {
		im := tinyImage(name)
		switch name {
		case "users":
			im = tinyImage(name, []string{s.Users[0], "user1"}, []string{s.Users[1], "user2"})
		case "profiles":
			im = tinyImage(name, []string{s.Profiles[0], "profile1"}, []string{s.Profiles[1], "profile2"})
		case "auth_login_identities":
			im = tinyImage(name, []string{s.LoginIdentities[0], s.Users[0]}, []string{s.LoginIdentities[1], s.Users[1]})
			im.Columns[1].Name = "user_id"
		}
		a.Tables = append(a.Tables, im)
		im.Rows = []row{}
		a.Protected = append(a.Protected, im)
	}
	f.o.archive = f.put(t, "archive", encode(a))
	f.o.catalog = f.put(t, "catalog", []byte("synthetic catalog not parsed before credentials\n"))
	f.app = approval{Format: 1, Kind: "mock_cohort_iam_approval", Action: "apply", Operation: g.Operation, Source: strings.Repeat("e", 40), GroupSHA: groupSHA, ArchiveSHA: f.o.archive.sha, FrozenSHA: f.o.frozen.sha, CatalogSHA: f.o.catalog.sha, Identity: id, Issued: now.Add(-time.Second), Expires: now.Add(time.Minute), Deadline: now.Add(2 * time.Minute)}
	f.proof = qsClearanceExecution{Format: 1, Kind: "mock_cohort_clearance_execution", Termination: "exited", Exit: 0}
	f.bind(t)
	return f
}

func TestQSClearanceAdmissionPrecedesCredentialsAndKeepsOriginals(t *testing.T) {
	for _, tc := range []struct {
		name  string
		edit  func(*clearanceFileFixture)
		reads int
	}{
		{"valid-independent-clearance-source", func(*clearanceFileFixture) {}, 1},
		{"failed-process", func(f *clearanceFileFixture) { f.proof.Exit = 1 }, 0},
		{"unknown-termination", func(f *clearanceFileFixture) { f.proof.Termination = "unknown" }, 0},
		{"expired", func(f *clearanceFileFixture) { f.c.Expires = f.now.Add(-time.Nanosecond) }, 0},
		{"not-complete", func(f *clearanceFileFixture) { f.c.Complete = false }, 0},
		{"different-raw-iam", func(f *clearanceFileFixture) { f.c.Batches[0].IAMSHA = strings.Repeat("c", 64) }, 0},
		{"different-freeze", func(f *clearanceFileFixture) { f.c.Batches[0].CurrentFrozenSHA = strings.Repeat("c", 64) }, 0},
		{"approval-exceeds-clearance", func(f *clearanceFileFixture) { f.app.Deadline = f.c.Expires.Add(time.Nanosecond) }, 0},
		{"capture-apply-source-mismatch", func(f *clearanceFileFixture) {
			var action qsActionReceipt
			raw, err := load(f.o.qsApply)
			if err != nil || strictJSON(raw, &action) != nil {
				t.Fatal(err)
			}
			action.Source = strings.Repeat("f", 40)
			f.o.qsApply = f.put(t, "different-source-apply", encode(action))
			f.h.PartialApplySHA, f.c.Batches[0].PartialApplySHA = f.o.qsApply.sha, f.o.qsApply.sha
		}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newClearanceFileFixture(t)
			originalFile := f.o.qsApply
			before, err := load(originalFile)
			if err != nil {
				t.Fatal(err)
			}
			tc.edit(f)
			f.bind(t)
			reads := 0
			r := receipt{Source: f.app.Source}
			err = execute(f.o, &r, func(string) string { reads++; return "" })
			if err == nil || reads != tc.reads {
				t.Fatalf("admission boundary differs: reads=%d want=%d err=%v", reads, tc.reads, err)
			}
			if tc.reads == 1 && err.Error() != "explicit IAM MySQL DSN required" {
				t.Fatal("valid v2 input did not reach the credential boundary", err)
			}
			var original qsActionReceipt
			if strictJSON(before, &original) != nil || original.Whole || !original.SemanticPending {
				t.Fatal("partial apply original relabelled")
			}
			after, readErr := load(originalFile)
			if readErr != nil || string(before) != string(after) {
				t.Fatal("historical partial apply original changed", readErr)
			}
		})
	}
}

func TestQSClearanceExecutionRequiresExactReceiptPathAndLiteralExit(t *testing.T) {
	for _, change := range []func(map[string]any){
		func(p map[string]any) { p["receipt_file"] = "/other/original" },
		func(p map[string]any) { p["receipt_sha256"] = strings.Repeat("c", 64) },
		func(p map[string]any) { p["exit_code"] = nil },
		func(p map[string]any) { delete(p, "exit_code") },
	} {
		f := newClearanceFileFixture(t)
		var p map[string]any
		if err := json.Unmarshal(encode(f.proof), &p); err != nil {
			t.Fatal(err)
		}
		change(p)
		f.o.qsClearanceExecution = f.put(t, "bad-execution", encode(p))
		f.h.ClearanceExecutionSHA = f.o.qsClearanceExecution.sha
		if _, _, err := loadQSClearance(f.o, encode(f.h), f.g, f.s, f.h.GroupSHA, f.now); err == nil {
			t.Fatal("unbound or missing successful original process evidence admitted")
		}
	}
}
