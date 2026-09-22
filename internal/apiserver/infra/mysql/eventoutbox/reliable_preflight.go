package eventoutbox

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"math"
	"time"

	"github.com/FangcunMount/iam/v5/internal/pkg/timezone"
	"gorm.io/gorm"
)

// ReliablePreflightReport deliberately contains no message identities, payloads,
// tokens or driver errors. Data readiness never proves cross-process exclusion.
type ReliablePreflightReport struct {
	DatabaseTime               string `json:"database_time"`
	Inspected                  int64  `json:"inspected"`
	Truncated                  bool   `json:"truncated"`
	Pending                    int64  `json:"pending"`
	Failed                     int64  `json:"failed"`
	Published                  int64  `json:"published"`
	Publishing                 int64  `json:"publishing"`
	Quarantined                int64  `json:"quarantined"`
	UnknownStatus              int64  `json:"unknown_status"`
	LegacyPublishing           int64  `json:"legacy_publishing"`
	ActiveSDKLeases            int64  `json:"active_sdk_leases"`
	ExpiredSDKLeases           int64  `json:"expired_sdk_leases"`
	InvalidUnfinishedIntents   int64  `json:"invalid_unfinished_intents"`
	UnfinishedContentConflicts int64  `json:"unfinished_content_conflicts"`
	PublishedContentIssues     int64  `json:"published_content_issues"`
	InconsistentMetadata       int64  `json:"inconsistent_metadata"`
	RowsWithoutFingerprint     int64  `json:"rows_without_fingerprint"`
	SDKMetadataUsed            int64  `json:"sdk_metadata_used"`
	SDKDataReady               bool   `json:"sdk_data_ready"`
	RecoveryRequired           bool   `json:"recovery_required"`
	LegacyRollbackDataReady    bool   `json:"legacy_rollback_data_ready"`
	UnusedSchemaCanBeRemoved   bool   `json:"unused_schema_can_be_removed"`
	WriterExclusionVerified    bool   `json:"writer_exclusion_verified"`
	CutoverAuthorized          bool   `json:"cutover_authorized"`
}

// InspectReliableOutbox reads a bounded repeatable-read, read-only snapshot.
// Caller must separately stop/verify all writers, retain this report and repeat
// it immediately before handoff. No records are claimed, rewritten or replayed.
// A full report requires migration 38 and enough row/time budget; partial reports
// never indicate readiness. Published rows are never made retry candidates.
func InspectReliableOutbox(ctx context.Context, db *gorm.DB, topic string, maxRows int) (ReliablePreflightReport, error) {
	var report ReliablePreflightReport
	if topic == "" || maxRows < 1 || maxRows > 1_000_000 {
		return report, errors.New("policy topic and row bound 1..1000000 required")
	}
	if err := CheckReliableSchema(ctx, db); err != nil {
		return report, err
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var databaseNow time.Time
		if err := tx.Raw("SELECT UTC_TIMESTAMP(6)").Row().Scan(&databaseNow); err != nil {
			return err
		}
		utcClock := time.Date(databaseNow.Year(), databaseNow.Month(), databaseNow.Day(), databaseNow.Hour(), databaseNow.Minute(), databaseNow.Second(), databaseNow.Nanosecond(), time.UTC)
		report.DatabaseTime = utcClock.In(timezone.Location).Format(time.RFC3339Nano)
		var cursor uint64
		firstPage := true
		for {
			var rows []reliableRow
			// One extra row distinguishes complete from bounded/truncated output.
			limit := min(200, maxRows-int(report.Inspected)+1)
			query := tx.Order("id").Limit(limit)
			if !firstPage {
				query = query.Where("id > ?", cursor)
			}
			if err := query.Find(&rows).Error; err != nil {
				return err
			}
			firstPage = false
			for _, row := range rows {
				if report.Inspected == int64(maxRows) {
					report.Truncated = true
					return nil
				}
				report.inspect(row, topic, databaseNow)
				cursor = row.ID
			}
			if len(rows) < limit {
				return nil
			}
		}
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return report, err
	}
	clean := !report.Truncated && report.UnknownStatus == 0 && report.Quarantined == 0 &&
		report.InvalidUnfinishedIntents == 0 && report.UnfinishedContentConflicts == 0 && report.InconsistentMetadata == 0
	// SDK can recover valid historical/expired claims after verified exclusion;
	// legacy rollback must first settle all publishing rows with the SDK. Neither
	// condition proves that another process or underlying send has terminated.
	report.SDKDataReady = clean
	report.RecoveryRequired = report.Publishing != 0
	report.LegacyRollbackDataReady = clean && report.Publishing == 0
	report.UnusedSchemaCanBeRemoved = !report.Truncated && report.SDKMetadataUsed == 0 && report.UnknownStatus == 0 && report.Quarantined == 0
	return report, nil
}

func (r *ReliablePreflightReport) inspect(row reliableRow, topic string, now time.Time) {
	r.Inspected++
	switch row.Status {
	case "pending":
		r.Pending++
	case "failed":
		r.Failed++
	case "published":
		r.Published++
	case "publishing":
		r.Publishing++
	case "quarantined":
		r.Quarantined++
	default:
		r.UnknownStatus++
	}
	if row.Fingerprint == nil {
		r.RowsWithoutFingerprint++
	}
	if row.ClaimToken != nil || row.LeaseUntil != nil || row.ClaimVersion != 0 || row.ClaimCount != 0 || row.Fingerprint != nil {
		r.SDKMetadataUsed++
	}
	badMetadata := row.ID == 0 || row.ClaimCount > row.ClaimVersion || row.ClaimCount == math.MaxUint64 || row.ClaimVersion == math.MaxUint64
	if row.Status == "publishing" {
		if row.ClaimToken == nil && row.LeaseUntil == nil {
			r.LegacyPublishing++
		} else if row.ClaimToken == nil || row.LeaseUntil == nil {
			badMetadata = true
		} else {
			token, err := hex.DecodeString(*row.ClaimToken)
			if err != nil || len(token) != 16 || row.ClaimCount == 0 || row.ClaimVersion == 0 {
				badMetadata = true
			}
			if row.LeaseUntil.After(now) {
				r.ActiveSDKLeases++
			} else {
				r.ExpiredSDKLeases++
			}
		}
	} else if row.ClaimToken != nil || row.LeaseUntil != nil {
		badMetadata = true
	}
	if badMetadata {
		r.InconsistentMetadata++
	}
	intent, err := policyIntent(row.OutboxPO, topic)
	fingerprint := intent.Fingerprint()
	conflict := err == nil && row.Fingerprint != nil && !bytes.Equal(row.Fingerprint, fingerprint[:])
	if row.Status == "published" {
		if err != nil || conflict {
			r.PublishedContentIssues++
		}
	} else if err != nil {
		r.InvalidUnfinishedIntents++
	} else if conflict {
		r.UnfinishedContentConflicts++
	}
}
