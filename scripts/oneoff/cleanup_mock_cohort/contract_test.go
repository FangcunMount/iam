package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func groupFixture(t *testing.T) ([]byte, []byte, []byte, groupManifest, iamScope) {
	t.Helper()
	s := iamScope{[]string{"101", "102"}, []string{"201", "202"}, []string{"301", "302"}}
	qs := json.RawMessage(`{"selector":{"source":"daily_simulation","org_id":"1"}}`)
	p := encode(protection{1, "mock_cohort_protected", iamScope{[]string{}, []string{}, []string{}}, json.RawMessage(`{"system":[],"clack":[],"other":[]}`)})
	o := encode(ownerConfirmation{1, "mock_cohort_owner_confirmation", "batch-one", "10001", jsonSHA(s), rawSHA(qs), rawSHA(p), true, true, []string{"1"}})
	g := groupManifest{1, "mock_cohort", "batch-one", rawSHA(o), rawSHA(p), json.RawMessage(mustMarshal(s)), qs}
	raw := encode(g)
	return raw, o, p, g, s
}
func mustMarshal(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}
func TestGroupOwnerWholeCohortAndTypedProtection(t *testing.T) {
	raw, o, p, g, s := groupFixture(t)
	if _, _, e := validateGroup(raw, o, p); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name string
		edit func(*groupManifest, *iamScope)
	}{
		{"system", func(_ *groupManifest, s *iamScope) { s.Users = []string{"10001"} }},
		{"clack", func(_ *groupManifest, s *iamScope) { s.Users = []string{"618855329043591726"} }},
		{"clack-profile", func(_ *groupManifest, s *iamScope) { s.Profiles = []string{"632367179001508398"} }},
		{"duplicate", func(_ *groupManifest, s *iamScope) { s.Users = []string{"101", "101"} }},
		{"leadingzero", func(_ *groupManifest, s *iamScope) { s.Users = []string{"0101"} }},
		{"overflow", func(_ *groupManifest, s *iamScope) { s.Users = []string{"9223372036854775808"} }},
		{"null", func(_ *groupManifest, s *iamScope) { s.LoginIdentities = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gg, ss := g, s
			tc.edit(&gg, &ss)
			gg.IAM = mustMarshal(ss)
			var oo ownerConfirmation
			if e := json.Unmarshal(o, &oo); e != nil {
				t.Fatal(e)
			}
			oo.IAMSHA = rawSHA(gg.IAM)
			oraw := encode(oo)
			gg.OwnerSHA = rawSHA(oraw)
			if _, _, e := validateGroup(encode(gg), oraw, p); e == nil {
				t.Fatal("unsafe cohort admitted")
			}
		})
	}
	// The same numeral in another namespace is not itself a user reference.
	s.Profiles = []string{"10001"}
	g.IAM = mustMarshal(s)
	var oo ownerConfirmation
	_ = json.Unmarshal(o, &oo)
	oo.IAMSHA = rawSHA(g.IAM)
	o = encode(oo)
	g.OwnerSHA = rawSHA(o)
	if _, _, e := validateGroup(encode(g), o, p); e != nil {
		t.Fatal(e)
	}
}
func TestSharedProtectionEnvelopeGolden(t *testing.T) {
	raw, e := os.ReadFile(filepath.Join("testdata", "shared-protected-v1.json"))
	if e != nil {
		t.Fatal(e)
	}
	var p protection
	if e = strictJSON(raw, &p); e != nil {
		t.Fatal(e)
	}
	if p.Kind != "mock_cohort_protected" || !contains(p.IAM.Users, "10001") || !contains(p.IAM.Users, "618855329043591726") {
		t.Fatal("shared protection lost typed defaults")
	}
	if !bytes.Equal(encode(p), raw) {
		t.Fatal("shared golden wire bytes changed")
	}
}
func TestStrictLiteralDocument(t *testing.T) {
	for _, raw := range []string{`{"users":1,"profiles":2,"login_identities":3}`, `{ "login_identities":3, "profiles":2,"users":1 }`} {
		var c counts
		if e := strictJSON([]byte(raw), &c); e != nil {
			t.Fatal(e)
		}
	}
	for _, raw := range []string{`{"Users":1,"profiles":2,"login_identities":3}`, `{"users":1,"users":1,"profiles":2,"login_identities":3}`, `{"users":null,"profiles":2,"login_identities":3}`, `{"profiles":2,"login_identities":3}`, `{"users":1,"profiles":2,"login_identities":3,"extra":0}`, `{"users":1,"profiles":2,"login_identities":3} {}`} {
		t.Run(raw, func(t *testing.T) {
			var c counts
			if e := strictJSON([]byte(raw), &c); e == nil {
				t.Fatal("nonliteral document admitted")
			}
		})
	}
}
func TestHandoffBindsRawGroupAndNamespaces(t *testing.T) {
	raw, _, _, g, s := groupFixture(t)
	sha := strings.Repeat("a", 64)
	h := qsHandoff{Format: 1, Kind: "mock_cohort_qs_cleared", Operation: g.Operation, GroupSHA: rawSHA(raw), OwnerSHA: g.OwnerSHA, ProtectedSHA: g.ProtectedSHA, IAMSHA: rawSHA(g.IAM), QSSHA: rawSHA(g.QS), IAMCounts: s.counts(), CaptureSHA: sha, ApplySHA: sha, VerifySHA: sha, SQLIdentity: json.RawMessage(`{"db":"qs"}`), MongoIdentity: json.RawMessage(`{"db":"qs"}`), SQLVersion: 101, MongoVersion: 39, ReferencesClear: true, FrozenSHA: sha, MessagesSHA: sha}
	if _, e := validateHandoff(encode(h), g, s, rawSHA(raw)); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*qsHandoff){func(h *qsHandoff) { h.GroupSHA = sha }, func(h *qsHandoff) { h.IAMSHA = sha }, func(h *qsHandoff) { h.IAMCounts.Users++ }, func(h *qsHandoff) { h.ReferencesClear = false }, func(h *qsHandoff) { h.Dirty = true }, func(h *qsHandoff) { h.MessagesSHA = "" }, func(h *qsHandoff) { h.MongoIdentity = nil }} {
		bad := h
		edit(&bad)
		if _, e := validateHandoff(encode(bad), g, s, rawSHA(raw)); e == nil {
			t.Fatal("handoff mismatch admitted")
		}
	}
}
func TestQSOriginalReceiptsRejectPendingOrFailedWholeGraph(t *testing.T) {
	r := qsActionReceipt{Format: 1, Kind: "mock_cohort_qs_action_result", Action: "verify", Operation: "op", GroupSHA: strings.Repeat("a", 64), Source: strings.Repeat("b", 40), MaterialSHA: strings.Repeat("c", 64), Complete: true, Whole: true, Released: true}
	if _, e := validateQSAction(encode(r), "verify", "op", r.GroupSHA); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*qsActionReceipt){func(r *qsActionReceipt) { r.Complete = false }, func(r *qsActionReceipt) { r.Whole = false }, func(r *qsActionReceipt) { r.SemanticPending = true }, func(r *qsActionReceipt) { r.DryRun = true }, func(r *qsActionReceipt) { r.Excluded = 1 }, func(r *qsActionReceipt) { r.Released = false }, func(r *qsActionReceipt) { r.Operation = "other" }, func(r *qsActionReceipt) { r.Source = "" }} {
		bad := r
		edit(&bad)
		if _, e := validateQSAction(encode(bad), "verify", "op", r.GroupSHA); e == nil {
			t.Fatal("incomplete QS evidence admitted")
		}
	}
	r.Action = "capture"
	r.Whole = false
	if _, e := validateQSAction(encode(r), "capture", "op", r.GroupSHA); e != nil {
		t.Fatal("complete material capture need not claim graph deletion", e)
	}
}
func TestApprovalExpiryAndIdentity(t *testing.T) {
	now := time.Now().UTC()
	id := identity{Database: "iam", ServerUUID: "uuid", Version: 40, RedisSHA: strings.Repeat("b", 64)}
	a := approval{Format: 1, Kind: "mock_cohort_iam_approval", Action: "apply", Source: "source", Operation: "op", GroupSHA: "g", ArchiveSHA: "a", HandoffSHA: "h", FrozenSHA: "f", Identity: id, Issued: now.Add(-time.Minute), Expires: now.Add(time.Minute), Deadline: now.Add(20 * time.Minute)}
	valid := func(a approval) error {
		return validateApproval(a, "apply", "source", "op", "g", "a", "h", "f", id, now)
	}
	if e := valid(a); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*approval){func(a *approval) { a.Identity.Database = "other" }, func(a *approval) { a.Identity.RedisSHA = "other" }, func(a *approval) { a.Expires = now }, func(a *approval) { a.Issued = now.Add(time.Second) }, func(a *approval) { a.Deadline = now.Add(31 * time.Minute) }, func(a *approval) { a.Source = "other" }, func(a *approval) { a.Action = "restore" }} {
		b := a
		edit(&b)
		if valid(b) == nil {
			t.Fatal("invalid current approval admitted")
		}
	}
}
func TestPrivateOriginalsAndExclusiveOutputs(t *testing.T) {
	d := t.TempDir()
	_ = os.Chmod(d, 0700)
	p := filepath.Join(d, "original.json")
	raw := []byte("raw\n")
	if e := writeExclusive(p, raw); e != nil {
		t.Fatal(e)
	}
	if _, e := readPrivate(p, rawSHA(raw), 100); e != nil {
		t.Fatal(e)
	}
	if e := writeExclusive(p, raw); e == nil {
		t.Fatal("overwrote original")
	}
	if _, e := readPrivate(p, rawSHA(raw), 2); e == nil {
		t.Fatal("ignored bound")
	}
	if _, e := readPrivate(p, strings.Repeat("a", 64), 100); e == nil {
		t.Fatal("accepted changed hash")
	}
	link := filepath.Join(d, "link")
	_ = os.Symlink(p, link)
	if _, e := readPrivate(link, rawSHA(raw), 100); e == nil {
		t.Fatal("followed symlink")
	}
	hard := filepath.Join(d, "hard")
	_ = os.Link(p, hard)
	if _, e := readPrivate(p, rawSHA(raw), 100); e == nil {
		t.Fatal("accepted hardlink")
	}
	_ = os.Remove(hard)
	_ = os.Chmod(p, 0644)
	if _, e := readPrivate(p, rawSHA(raw), 100); e == nil {
		t.Fatal("accepted public original")
	}
}
func TestMalformedInputsDoNotReadCredentials(t *testing.T) {
	for _, action := range []string{"plan", "capture", "apply", "verify", "restore"} {
		t.Run(action, func(t *testing.T) {
			var out bytes.Buffer
			reads := 0
			exit := serve([]string{"--action", action}, &out, func(string) string { reads++; return "secret" })
			if exit == 0 || reads != 0 || strings.Contains(out.String(), "secret") {
				t.Fatal("bad input crossed credential boundary")
			}
		})
	}
}
