package main

import (
	"encoding/json"
	"errors"
	"regexp"
	"time"
)

type qsDatabaseBinding struct {
	SQLUUID       string `json:"mysql_server_uuid"`
	SQLDatabase   string `json:"mysql_database"`
	MongoIdentity string `json:"mongo_identity_sha256"`
	MongoDatabase string `json:"mongo_database"`
	SQLVersion    int64  `json:"mysql_version"`
	MongoVersion  int64  `json:"mongo_version"`
}

type qsBatchHandoff struct {
	Format                   int    `json:"format_version"`
	Kind                     string `json:"kind"`
	Operation                string `json:"operation_id"`
	GroupSHA                 string `json:"group_manifest_sha256"`
	OwnerSHA                 string `json:"owner_confirmation_sha256"`
	ProtectedSHA             string `json:"protected_manifest_sha256"`
	IAMSHA                   string `json:"iam_sha256"`
	QSSHA                    string `json:"qs_sha256"`
	IAMCounts                counts `json:"iam_counts"`
	BatchManifestSHA         string `json:"batch_manifest_sha256"`
	CaptureSHA               string `json:"qs_capture_sha256"`
	PartialApplySHA          string `json:"qs_partial_apply_sha256"`
	PartialApplyExecutionSHA string `json:"qs_partial_apply_execution_sha256"`
	ClearanceSHA             string `json:"qs_clearance_sha256"`
	ClearanceExecutionSHA    string `json:"qs_clearance_execution_sha256"`
	CurrentFrozenSHA         string `json:"current_frozen_receipt_sha256"`
}

type qsClearanceBatch struct {
	GroupSHA                 string `json:"group_manifest_sha256"`
	OwnerSHA                 string `json:"owner_confirmation_sha256"`
	IAMSHA                   string `json:"iam_sha256"`
	QSSHA                    string `json:"qs_sha256"`
	IAMCounts                counts `json:"iam_counts"`
	MaterialSHA              string `json:"material_sha256"`
	CaptureSHA               string `json:"qs_capture_sha256"`
	CaptureEvidenceSHA       string `json:"capture_evidence_sha256"`
	PartialApplySHA          string `json:"qs_partial_apply_sha256"`
	PartialApplyExecutionSHA string `json:"qs_partial_apply_execution_sha256"`
	JournalSHA               string `json:"journal_sha256"`
	CurrentFrozenSHA         string `json:"current_frozen_receipt_sha256"`
}

type qsClearanceReceipt struct {
	Format          int                `json:"format_version"`
	Kind            string             `json:"kind"`
	Action          string             `json:"action"`
	Operation       string             `json:"operation_id"`
	Source          string             `json:"compiled_source"`
	BatchSHA        string             `json:"batch_manifest_sha256"`
	ProtectedSHA    string             `json:"protected_manifest_sha256"`
	Binding         qsDatabaseBinding  `json:"binding"`
	Started         time.Time          `json:"observed_started_at"`
	Completed       time.Time          `json:"observed_completed_at"`
	Expires         time.Time          `json:"expires_at"`
	Batches         []qsClearanceBatch `json:"batches"`
	Absent          bool               `json:"targets_absent"`
	Protected       bool               `json:"protected_unchanged"`
	ReferencesClear bool               `json:"global_references_clear"`
	Complete        bool               `json:"complete"`
	Released        bool               `json:"resource_release_proven"`
}

type qsClearanceExecution struct {
	Format      int    `json:"format_version"`
	Kind        string `json:"kind"`
	ReceiptFile string `json:"receipt_file"`
	ReceiptSHA  string `json:"receipt_sha256"`
	Termination string `json:"termination"`
	Exit        int    `json:"exit_code"`
}

// Clearance evidence is admitted only through both explicit original files.
// An incomplete opt-in must never fall through to the historical handoff path.
func qsClearanceMode(o options) (bool, error) {
	if o.qsClearance == (inputFile{}) && o.qsClearanceExecution == (inputFile{}) {
		return false, nil
	}
	for _, in := range []inputFile{o.qsClearance, o.qsClearanceExecution} {
		if in.path == "" || !digestPattern.MatchString(in.sha) {
			return false, errors.New("both QS clearance originals and raw SHA required")
		}
	}
	if o.action != "apply" && o.action != "restore" {
		return false, errors.New("QS clearance only belongs to IAM apply or restore")
	}
	if o.qsVerify != (inputFile{}) {
		return false, errors.New("QS clearance cannot be mixed with original verify evidence")
	}
	return true, nil
}

func validateQSBatchHandoff(raw []byte, g groupManifest, s iamScope, groupSHA string) (qsBatchHandoff, error) {
	var h qsBatchHandoff
	if err := strictJSON(raw, &h); err != nil {
		return h, err
	}
	if h.Format != 1 || h.Kind != "mock_cohort_qs_batch_cleared" || h.Operation != g.Operation || h.GroupSHA != groupSHA || h.OwnerSHA != g.OwnerSHA || h.ProtectedSHA != g.ProtectedSHA || h.IAMSHA != rawSHA(g.IAM) || h.QSSHA != rawSHA(g.QS) || h.IAMCounts != s.counts() {
		return h, errors.New("QS batch handoff binding rejected")
	}
	for _, sha := range []string{h.BatchManifestSHA, h.CaptureSHA, h.PartialApplySHA, h.PartialApplyExecutionSHA, h.ClearanceSHA, h.ClearanceExecutionSHA, h.CurrentFrozenSHA} {
		if !digestPattern.MatchString(sha) {
			return h, errors.New("QS batch evidence digest missing")
		}
	}
	return h, nil
}

func validateQSClearance(raw []byte, g groupManifest, s iamScope, groupSHA string, h qsBatchHandoff, now time.Time) (qsClearanceReceipt, qsClearanceBatch, error) {
	var c qsClearanceReceipt
	var member qsClearanceBatch
	if err := strictJSON(raw, &c); err != nil {
		return c, member, err
	}
	var qs map[string]json.RawMessage
	var binding qsDatabaseBinding
	if err := strictJSON(g.QS, &qs); err != nil {
		return c, member, err
	}
	if err := strictJSON(qs["binding"], &binding); err != nil {
		return c, member, err
	}
	if c.Format != 1 || c.Kind != "mock_cohort_qs_clearance" || c.Action != "clearance" || c.Operation != g.Operation || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(c.Source) || c.BatchSHA != h.BatchManifestSHA || c.ProtectedSHA != g.ProtectedSHA || c.Binding != binding || binding.SQLUUID == "" || binding.SQLDatabase == "" || !digestPattern.MatchString(binding.MongoIdentity) || binding.MongoDatabase == "" || binding.SQLVersion != 101 || binding.MongoVersion != 39 || !c.Absent || !c.Protected || !c.ReferencesClear || !c.Complete || !c.Released || len(c.Batches) == 0 {
		return c, member, errors.New("QS current clearance binding rejected")
	}
	if err := validateQSClearanceTime(c, now); err != nil {
		return c, member, err
	}
	seen := map[string]bool{}
	found := false
	for _, b := range c.Batches {
		for _, sha := range []string{b.GroupSHA, b.OwnerSHA, b.IAMSHA, b.QSSHA, b.MaterialSHA, b.CaptureSHA, b.CaptureEvidenceSHA, b.PartialApplySHA, b.PartialApplyExecutionSHA, b.JournalSHA, b.CurrentFrozenSHA} {
			if !digestPattern.MatchString(sha) {
				return c, member, errors.New("QS clearance batch evidence digest missing")
			}
		}
		if seen[b.GroupSHA] || b.IAMCounts.Users < 0 || b.IAMCounts.Profiles < 0 || b.IAMCounts.LoginIdentities < 0 {
			return c, member, errors.New("QS clearance batch identity ambiguous")
		}
		seen[b.GroupSHA] = true
		if b.GroupSHA != groupSHA {
			continue
		}
		if b.OwnerSHA != g.OwnerSHA || b.IAMSHA != rawSHA(g.IAM) || b.QSSHA != rawSHA(g.QS) || b.IAMCounts != s.counts() || b.CaptureSHA != h.CaptureSHA || b.PartialApplySHA != h.PartialApplySHA || b.PartialApplyExecutionSHA != h.PartialApplyExecutionSHA || b.CurrentFrozenSHA != h.CurrentFrozenSHA {
			return c, member, errors.New("QS clearance exact batch differs")
		}
		member, found = b, true
	}
	if !found {
		return c, member, errors.New("QS clearance batch missing")
	}
	return c, member, nil
}

func validateQSClearanceTime(c qsClearanceReceipt, now time.Time) error {
	if c.Started.IsZero() || c.Completed.IsZero() || c.Expires.IsZero() || c.Completed.Before(c.Started) || !c.Completed.Before(c.Expires) || c.Expires.Sub(c.Started) > 25*time.Minute || now.Before(c.Completed) || !now.Before(c.Expires) {
		return errors.New("QS current clearance time rejected")
	}
	return nil
}

func validateQSClearanceApproval(c qsClearanceReceipt, a approval, now time.Time) error {
	if err := validateQSClearanceTime(c, now); err != nil {
		return err
	}
	if a.Expires.After(c.Expires) || a.Deadline.After(c.Expires) {
		return errors.New("IAM approval exceeds QS current clearance")
	}
	return nil
}

func validateQSPartialApply(raw []byte, op, groupSHA string) (qsActionReceipt, error) {
	var r qsActionReceipt
	if err := strictJSON(raw, &r); err != nil {
		return r, err
	}
	if r.Format != 1 || r.Kind != "mock_cohort_qs_action_result" || r.Action != "apply" || r.Operation != op || r.GroupSHA != groupSHA || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(r.Source) || !digestPattern.MatchString(r.MaterialSHA) || !r.Complete || r.DryRun || r.Whole || !r.SemanticPending || r.Excluded != 0 || !r.Released {
		return r, errors.New("QS partial apply original is not complete batch evidence")
	}
	return r, nil
}

func loadQSClearance(o options, hRaw []byte, g groupManifest, s iamScope, groupSHA string, now time.Time) (qsClearanceReceipt, string, error) {
	var c qsClearanceReceipt
	h, err := validateQSBatchHandoff(hRaw, g, s, groupSHA)
	if err != nil {
		return c, "", err
	}
	for _, x := range []struct{ actual, expected string }{{o.qsCapture.sha, h.CaptureSHA}, {o.qsApply.sha, h.PartialApplySHA}, {o.qsClearance.sha, h.ClearanceSHA}, {o.qsClearanceExecution.sha, h.ClearanceExecutionSHA}, {o.frozen.sha, h.CurrentFrozenSHA}, {o.messages.sha, h.CurrentFrozenSHA}} {
		if x.actual != x.expected {
			return c, "", errors.New("QS batch original evidence SHA differs")
		}
	}
	raw, err := load(o.qsClearance)
	if err != nil {
		return c, "", err
	}
	c, member, err := validateQSClearance(raw, g, s, groupSHA, h, now)
	if err != nil {
		return c, "", err
	}
	raw, err = load(o.qsClearanceExecution)
	if err != nil {
		return c, "", err
	}
	var execution qsClearanceExecution
	if err = strictJSON(raw, &execution); err != nil {
		return c, "", err
	}
	if execution.Format != 1 || execution.Kind != "mock_cohort_clearance_execution" || execution.ReceiptFile != o.qsClearance.path || execution.ReceiptSHA != o.qsClearance.sha || execution.Termination != "exited" || execution.Exit != 0 {
		return c, "", errors.New("QS clearance actual successful process exit missing")
	}
	var capture qsActionReceipt
	for i, in := range []inputFile{o.qsCapture, o.qsApply} {
		raw, err = load(in)
		if err != nil {
			return c, "", err
		}
		var action qsActionReceipt
		if i == 0 {
			action, err = validateQSAction(raw, "capture", g.Operation, groupSHA)
			capture = action
		} else {
			action, err = validateQSPartialApply(raw, g.Operation, groupSHA)
		}
		if err != nil {
			return c, "", err
		}
		if action.MaterialSHA != member.MaterialSHA || action.Source != capture.Source {
			return c, "", errors.New("QS batch action material or source differs")
		}
	}
	freezeRaw, err := load(o.frozen)
	if err != nil {
		return c, "", err
	}
	messagesRaw, err := load(o.messages)
	if err != nil {
		return c, "", err
	}
	if rawSHA(freezeRaw) != rawSHA(messagesRaw) {
		return c, "", errors.New("QS current freeze and settlement originals differ")
	}
	return c, h.CurrentFrozenSHA, nil
}
