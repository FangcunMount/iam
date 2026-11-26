// Command cleanup_mock_cohort is an offline, exact-manifest IAM maintenance tool.
// It neither discovers a cohort from names nor exposes an online deletion API.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"syscall"
	"time"
)

const maxDocument = 256 << 20

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var operationPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,95}$`)

type iamScope struct {
	Users           []string `json:"user_ids"`
	Profiles        []string `json:"profile_ids"`
	LoginIdentities []string `json:"login_identity_ids"`
}
type groupManifest struct {
	Format       int             `json:"format_version"`
	Kind         string          `json:"kind"`
	Operation    string          `json:"operation_id"`
	OwnerSHA     string          `json:"owner_confirmation_sha256"`
	ProtectedSHA string          `json:"protected_manifest_sha256"`
	IAM          json.RawMessage `json:"iam"`
	QS           json.RawMessage `json:"qs"`
}
type counts struct {
	Users           int `json:"users"`
	Profiles        int `json:"profiles"`
	LoginIdentities int `json:"login_identities"`
}

func (s iamScope) counts() counts {
	return counts{len(s.Users), len(s.Profiles), len(s.LoginIdentities)}
}

type qsHandoff struct {
	Format          int             `json:"format_version"`
	Kind            string          `json:"kind"`
	Operation       string          `json:"operation_id"`
	GroupSHA        string          `json:"group_manifest_sha256"`
	OwnerSHA        string          `json:"owner_confirmation_sha256"`
	ProtectedSHA    string          `json:"protected_manifest_sha256"`
	IAMSHA          string          `json:"iam_sha256"`
	QSSHA           string          `json:"qs_sha256"`
	IAMCounts       counts          `json:"iam_counts"`
	CaptureSHA      string          `json:"qs_capture_sha256"`
	ApplySHA        string          `json:"qs_apply_sha256"`
	VerifySHA       string          `json:"qs_verify_sha256"`
	SQLIdentity     json.RawMessage `json:"sql_identity"`
	MongoIdentity   json.RawMessage `json:"mongo_identity"`
	SQLVersion      uint64          `json:"sql_version"`
	MongoVersion    uint64          `json:"mongo_version"`
	Dirty           bool            `json:"dirty"`
	ReferencesClear bool            `json:"retained_references_clear"`
	FrozenSHA       string          `json:"frozen_receipt_sha256"`
	MessagesSHA     string          `json:"messages_settled_receipt_sha256"`
}
type qsActionReceipt struct {
	Format          int    `json:"format_version"`
	Kind            string `json:"kind"`
	Action          string `json:"action"`
	Operation       string `json:"operation_id"`
	GroupSHA        string `json:"group_manifest_sha256"`
	Source          string `json:"compiled_source"`
	MaterialSHA     string `json:"material_sha256"`
	Complete        bool   `json:"complete"`
	DryRun          bool   `json:"dry_run"`
	Whole           bool   `json:"whole_cohort_complete"`
	SemanticPending bool   `json:"semantic_graph_expansion_pending"`
	Excluded        int    `json:"excluded_components"`
	Released        bool   `json:"resource_release_proven"`
}

func validateQSAction(raw []byte, action, op, groupSHA string) (qsActionReceipt, error) {
	var r qsActionReceipt
	if err := strictJSON(raw, &r); err != nil {
		return r, err
	}
	if r.Format != 1 || r.Kind != "mock_cohort_qs_action_result" || r.Action != action || r.Operation != op || r.GroupSHA != groupSHA || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(r.Source) || !digestPattern.MatchString(r.MaterialSHA) || !r.Complete || r.DryRun || r.SemanticPending || r.Excluded != 0 || !r.Released || (action != "capture" && !r.Whole) {
		return r, errors.New("QS original action is not complete cohort evidence")
	}
	return r, nil
}

type protection struct {
	Format int             `json:"format_version"`
	Kind   string          `json:"kind"`
	IAM    iamScope        `json:"iam"`
	QS     json.RawMessage `json:"qs"`
}

// Owner confirmation names a whole dedicated cohort. Creation-log provenance and
// user_created are deliberately not admission rules.
type ownerConfirmation struct {
	Format         int      `json:"format_version"`
	Kind           string   `json:"kind"`
	Operation      string   `json:"operation_id"`
	Owner          string   `json:"owner_user_id"`
	IAMSHA         string   `json:"iam_sha256"`
	QSSHA          string   `json:"qs_sha256"`
	ProtectedSHA   string   `json:"protected_manifest_sha256"`
	Dedicated      bool     `json:"dedicated_mock_cohort"`
	SharedExcluded bool     `json:"real_and_shared_excluded"`
	AssignmentOrgs []string `json:"iam_assignment_org_ids"`
}
type identity struct {
	Database   string `json:"database"`
	ServerUUID string `json:"server_uuid"`
	Version    uint64 `json:"version"`
	Dirty      bool   `json:"dirty"`
	RedisSHA   string `json:"redis_identity_sha256"`
}
type approval struct {
	Format          int       `json:"format_version"`
	Kind            string    `json:"kind"`
	Action          string    `json:"action"`
	Operation       string    `json:"operation_id"`
	Source          string    `json:"compiled_source"`
	GroupSHA        string    `json:"group_manifest_sha256"`
	ArchiveSHA      string    `json:"archive_sha256"`
	HandoffSHA      string    `json:"qs_handoff_sha256"`
	FrozenSHA       string    `json:"frozen_receipt_sha256"`
	CatalogSHA      string    `json:"event_catalog_sha256"`
	PriorJournalSHA string    `json:"prior_journal_sha256"`
	Identity        identity  `json:"identity"`
	Issued          time.Time `json:"issued_at"`
	Expires         time.Time `json:"expires_at"`
	Deadline        time.Time `json:"window_deadline"`
}
type freezeReceipt struct {
	Format        int    `json:"format_version"`
	Kind          string `json:"kind"`
	Operation     string `json:"operation_id"`
	GroupSHA      string `json:"group_manifest_sha256"`
	IAMStopped    bool   `json:"all_iam_writers_stopped"`
	QSStopped     bool   `json:"all_qs_writers_stopped"`
	ReplaySettled bool   `json:"messages_and_manual_replay_settled"`
	Reload        bool   `json:"iam_projection_reload_before_reopen"`
}

func rawSHA(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func jsonSHA(v any) string   { b, _ := json.Marshal(v); return rawSHA(b) }
func validIDs(ids []string) error {
	if ids == nil {
		return errors.New("null ID list")
	}
	seen := map[string]bool{}
	for _, s := range ids {
		n, e := strconv.ParseUint(s, 10, 64)
		if e != nil || n == 0 || n > math.MaxInt64 || strconv.FormatUint(n, 10) != s || seen[s] {
			return errors.New("noncanonical or duplicate typed ID")
		}
		seen[s] = true
	}
	return nil
}
func (s iamScope) validate() error {
	for _, ids := range [][]string{s.Users, s.Profiles, s.LoginIdentities} {
		if e := validIDs(ids); e != nil {
			return e
		}
	}
	return nil
}
func overlap(a, b []string) bool {
	m := map[string]bool{}
	for _, s := range a {
		m[s] = true
	}
	for _, s := range b {
		if m[s] {
			return true
		}
	}
	return false
}
func validateGroup(raw, ownerRaw, protectedRaw []byte) (groupManifest, iamScope, error) {
	var g groupManifest
	var s iamScope
	var p protection
	var o ownerConfirmation
	for _, x := range []struct {
		raw []byte
		v   any
	}{{raw, &g}, {ownerRaw, &o}, {protectedRaw, &p}} {
		if e := strictJSON(x.raw, x.v); e != nil {
			return g, s, e
		}
	}
	if e := strictJSON(g.IAM, &s); e != nil {
		return g, s, e
	}
	if e := s.validate(); e != nil {
		return g, s, e
	}
	if e := p.IAM.validate(); e != nil {
		return g, s, e
	}
	var qsProtected struct {
		System []json.RawMessage `json:"system"`
		Clack  []json.RawMessage `json:"clack"`
		Other  []json.RawMessage `json:"other"`
	}
	if e := strictJSON(p.QS, &qsProtected); e != nil {
		return g, s, e
	}
	if qsProtected.System == nil || qsProtected.Clack == nil || qsProtected.Other == nil {
		return g, s, errors.New("QS protection lists required")
	}
	if o.AssignmentOrgs == nil {
		return g, s, errors.New("assignment organization scope required")
	}
	seenOrgs := map[string]bool{}
	for _, org := range o.AssignmentOrgs {
		if org != "0" {
			if e := validIDs([]string{org}); e != nil {
				return g, s, e
			}
		}
		if seenOrgs[org] {
			return g, s, errors.New("duplicate assignment organization")
		}
		seenOrgs[org] = true
	}
	if g.Format != 1 || g.Kind != "mock_cohort" || !operationPattern.MatchString(g.Operation) || len(g.QS) == 0 || bytes.Equal(g.QS, []byte("null")) || g.OwnerSHA != rawSHA(ownerRaw) || g.ProtectedSHA != rawSHA(protectedRaw) || p.Format != 1 || p.Kind != "mock_cohort_protected" {
		return g, s, errors.New("group binding rejected")
	}
	if o.Format != 1 || o.Kind != "mock_cohort_owner_confirmation" || o.Operation != g.Operation || o.Owner != "10001" || o.IAMSHA != rawSHA(g.IAM) || o.QSSHA != rawSHA(g.QS) || o.ProtectedSHA != g.ProtectedSHA || !o.Dedicated || !o.SharedExcluded {
		return g, s, errors.New("owner confirmation rejected")
	}
	if overlap(s.Users, append([]string{"10001", "618855329043591726"}, p.IAM.Users...)) || overlap(s.Profiles, append([]string{"632367179001508398", "636374014138266158"}, p.IAM.Profiles...)) || overlap(s.LoginIdentities, p.IAM.LoginIdentities) {
		return g, s, errors.New("protected identity in cohort")
	}
	return g, s, nil
}
func validateHandoff(raw []byte, g groupManifest, s iamScope, groupSHA string) (qsHandoff, error) {
	var h qsHandoff
	if e := strictJSON(raw, &h); e != nil {
		return h, e
	}
	if h.Format != 1 || h.Kind != "mock_cohort_qs_cleared" || h.Operation != g.Operation || h.GroupSHA != groupSHA || h.OwnerSHA != g.OwnerSHA || h.ProtectedSHA != g.ProtectedSHA || h.IAMSHA != rawSHA(g.IAM) || h.QSSHA != rawSHA(g.QS) || h.IAMCounts != s.counts() || h.Dirty || !h.ReferencesClear || h.SQLVersion == 0 || h.MongoVersion == 0 {
		return h, errors.New("QS handoff binding rejected")
	}
	for _, v := range []string{h.CaptureSHA, h.ApplySHA, h.VerifySHA, h.FrozenSHA, h.MessagesSHA} {
		if !digestPattern.MatchString(v) {
			return h, errors.New("QS evidence digest missing")
		}
	}
	for _, v := range []json.RawMessage{h.SQLIdentity, h.MongoIdentity} {
		if len(v) == 0 || bytes.Equal(v, []byte("null")) {
			return h, errors.New("QS database identity missing")
		}
	}
	return h, nil
}

// Strict literal field membership: encoding/json alone accepts case aliases and
// null bools. Compare decoded shape to the typed canonical shape, never rewrite
// the evidence bytes or their SHA.
func strictJSON(raw []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("trailing JSON")
	}
	// archive contains only concrete wire fields, with no opaque/custom JSON.
	// Exact canonical bytes therefore prove literal membership and uniqueness
	// without retaining two additional interface trees for large captures.
	if _, ok := v.(*archive); ok {
		if canonical, err := json.Marshal(v); err == nil && bytes.Equal(bytes.TrimSpace(raw), canonical) {
			return nil
		}
	}
	// Keep the original validation order for every noncanonical/other document.
	if e := uniqueJSON(raw); e != nil {
		return e
	}
	canonical, e := json.Marshal(v)
	if e != nil {
		return e
	}
	decode := func(b []byte) (any, error) {
		var x any
		d := json.NewDecoder(bytes.NewReader(b))
		d.UseNumber()
		e := d.Decode(&x)
		return x, e
	}
	a, e := decode(raw)
	if e != nil {
		return e
	}
	b, e := decode(canonical)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(a, b) {
		return errors.New("nonliteral or missing JSON member")
	}
	return nil
}
func uniqueJSON(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		t, e := d.Token()
		if e != nil {
			return e
		}
		del, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch del {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return e
				}
				s, ok := k.(string)
				if !ok || seen[s] {
					return errors.New("duplicate JSON key")
				}
				seen[s] = true
				if e = walk(); e != nil {
					return e
				}
			}
		case '[':
			for d.More() {
				if e = walk(); e != nil {
					return e
				}
			}
		default:
			return errors.New("unexpected JSON delimiter")
		}
		_, e = d.Token()
		return e
	}
	return walk()
}
func readPrivate(path, sha string, limit int64) (result []byte, retErr error) {
	if !filepath.IsAbs(path) || !digestPattern.MatchString(sha) {
		return nil, errors.New("private absolute file and raw SHA required")
	}
	fd, e := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), path)
	defer func() { retErr = errors.Join(retErr, f.Close()) }()
	st, e := f.Stat()
	if e != nil {
		return nil, e
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok || !st.Mode().IsRegular() || st.Mode().Perm() != 0600 || int(sys.Uid) != os.Getuid() || sys.Nlink != 1 || st.Size() > limit {
		return nil, errors.New("private file metadata rejected")
	}
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil || int64(len(b)) > limit {
		return nil, errors.New("bounded read failed")
	}
	if rawSHA(b) != sha {
		return nil, errors.New("raw SHA changed")
	}
	return b, nil
}
func writeExclusive(path string, b []byte) error {
	if !filepath.IsAbs(path) {
		return errors.New("absolute output path required")
	}
	st, e := os.Lstat(filepath.Dir(path))
	if e != nil {
		return e
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok || !st.IsDir() || st.Mode().Perm() != 0700 || int(sys.Uid) != os.Getuid() {
		return errors.New("private output parent required")
	}
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	n, w := f.Write(b)
	if n != len(b) {
		w = errors.Join(w, io.ErrShortWrite)
	}
	syncErr := f.Sync()
	closeErr := f.Close()
	dir, dirErr := os.Open(filepath.Dir(path))
	if dirErr == nil {
		dirErr = dir.Sync()
		dirErr = errors.Join(dirErr, dir.Close())
	}
	return errors.Join(w, syncErr, closeErr, dirErr)
}
func encode(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		panic(fmt.Sprintf("internal JSON encoding: %v", e))
	}
	return append(b, '\n')
}
