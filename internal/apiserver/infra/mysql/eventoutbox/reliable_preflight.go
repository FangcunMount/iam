package eventoutbox

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"math"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	"github.com/FangcunMount/iam/v5/internal/pkg/timezone"
	"github.com/FangcunMount/reliable-messaging/message"
	"gorm.io/gorm"
)

// Reports expose counts only. An OK data check never proves stopped writers,
// consumer convergence or production cutover authorization.
type ReliablePreflightReport struct {
	Schema                   string `json:"schema"`
	DatabaseTime             string `json:"database_time"`
	Inspected                int64  `json:"inspected"`
	Truncated                bool   `json:"truncated"`
	LegacyPublished          int64  `json:"legacy_published"`
	LegacyUnfinished         int64  `json:"legacy_unfinished"`
	LegacyUnknown            int64  `json:"legacy_unknown"`
	StandardRows             int64  `json:"standard_rows"`
	Pending                  int64  `json:"pending"`
	Failed                   int64  `json:"retry_wait"`
	Published                int64  `json:"published"`
	Publishing               int64  `json:"publishing"`
	Quarantined              int64  `json:"quarantined"`
	UnknownStatus            int64  `json:"unknown_status"`
	InvalidUnfinishedIntents int64  `json:"invalid_unfinished_intents"`
	PublishedContentIssues   int64  `json:"published_content_issues"`
	InconsistentMetadata     int64  `json:"inconsistent_metadata"`
	SDKDataReady             bool   `json:"sdk_data_ready"`
	RecoveryRequired         bool   `json:"recovery_required"`
	LegacyRollbackDataReady  bool   `json:"legacy_rollback_data_ready"`
	UnusedSchemaCanBeRemoved bool   `json:"unused_schema_can_be_removed"`
	WriterExclusionVerified  bool   `json:"writer_exclusion_verified"`
	CutoverAuthorized        bool   `json:"cutover_authorized"`
}

type standardPreflightRow struct {
	ID            uint64
	State         string
	Producer      string
	MessageID     string
	Destination   string
	EventType     string
	SchemaVersion string
	Scope         string
	ContentType   string
	OccurredAt    string
	Payload       []byte
	Fingerprint   []byte
	ClaimToken    []byte
	LeaseUntil    *string
	Version       uint64
	Attempts      uint64
}

// The combined row budget applies to both tables. No claim/update/delete is
// issued, and truncated or failed reads cannot authorize a data handoff.
func InspectReliableOutbox(ctx context.Context, db *gorm.DB, topic string, maxRows int) (ReliablePreflightReport, error) {
	report := ReliablePreflightReport{Schema: "iam-standard-outbox-v1"}
	if topic == "" || maxRows < 1 || maxRows > 1_000_000 {
		return report, errors.New("policy topic and row bound 1..1000000 required")
	}
	if err := CheckReliableSchema(ctx, db); err != nil {
		return report, err
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var nowText string
		if err := tx.Raw("SELECT DATE_FORMAT(UTC_TIMESTAMP(6),'%Y-%m-%d %H:%i:%s.%f')").Row().Scan(&nowText); err != nil {
			return err
		}
		now, err := time.Parse("2006-01-02 15:04:05.000000", nowText)
		if err != nil {
			return err
		}
		report.DatabaseTime = now.In(timezone.Location).Format(time.RFC3339Nano)
		for _, table := range []string{"domain_event_outbox", "rm_outbox"} {
			var cursor uint64
			first := true
			for {
				limit := min(200, maxRows-int(report.Inspected)+1)
				query := tx.Table(table).Order("id").Limit(limit)
				if !first {
					query = query.Where("id > ?", cursor)
				}
				first = false
				if table == "domain_event_outbox" {
					var rows []struct {
						ID     uint64
						Status string
					}
					if err := query.Select("id,status").Find(&rows).Error; err != nil {
						return err
					}
					for _, row := range rows {
						if report.Inspected == int64(maxRows) {
							report.Truncated = true
							return nil
						}
						report.Inspected++
						cursor = row.ID
						if row.Status == "published" {
							report.LegacyPublished++
						} else {
							report.LegacyUnfinished++
							if row.Status != "pending" && row.Status != "failed" && row.Status != "publishing" {
								report.LegacyUnknown++
							}
						}
					}
					if len(rows) < limit {
						break
					}
				} else {
					var rows []standardPreflightRow
					if err := query.Select("id,state,producer,message_id,destination,event_type,schema_version,scope,content_type,occurred_at,payload,fingerprint,claim_token,DATE_FORMAT(lease_until,'%Y-%m-%d %H:%i:%s.%f') AS lease_until,version,attempt_count AS attempts").Find(&rows).Error; err != nil {
						return err
					}
					for _, row := range rows {
						if report.Inspected == int64(maxRows) {
							report.Truncated = true
							return nil
						}
						report.inspectStandard(row, topic)
						cursor = row.ID
					}
					if len(rows) < limit {
						break
					}
				}
			}
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return report, err
	}
	clean := !report.Truncated && report.InconsistentMetadata == 0 && report.InvalidUnfinishedIntents == 0 && report.UnknownStatus == 0 && report.Quarantined == 0
	report.SDKDataReady = clean && report.LegacyUnfinished == 0
	report.RecoveryRequired = report.Pending+report.Failed+report.Publishing > 0
	// Old Relay does not read standard records, so every standard nonterminal
	// state must be settled before disabling SDK ownership.
	report.LegacyRollbackDataReady = clean && report.StandardRows == report.Published && report.LegacyUnknown == 0
	report.UnusedSchemaCanBeRemoved = !report.Truncated && report.StandardRows == 0
	return report, nil
}

func (r *ReliablePreflightReport) inspectStandard(row standardPreflightRow, topic string) {
	r.Inspected++
	r.StandardRows++
	switch row.State {
	case "pending":
		r.Pending++
	case "retry_wait":
		r.Failed++
	case "publishing":
		r.Publishing++
	case "published":
		r.Published++
	case "quarantined":
		r.Quarantined++
	default:
		r.UnknownStatus++
	}
	bad := row.ID == 0 || row.Attempts > row.Version || row.Attempts == math.MaxUint64 || row.Version == math.MaxUint64
	if row.State == "publishing" {
		token, err := hex.DecodeString(string(row.ClaimToken))
		if err != nil || len(token) != 32 || row.LeaseUntil == nil || row.Attempts == 0 || row.Version == 0 {
			bad = true
		}
		if row.LeaseUntil != nil {
			if _, err := time.Parse("2006-01-02 15:04:05.000000", *row.LeaseUntil); err != nil {
				bad = true
			}
		}
	} else if len(row.ClaimToken) != 0 || row.LeaseUntil != nil {
		bad = true
	}
	if bad {
		r.InconsistentMetadata++
	}
	intent, err := message.New(message.Input{Producer: row.Producer, ID: row.MessageID, Destination: row.Destination, EventType: row.EventType, SchemaVersion: row.SchemaVersion, Scope: row.Scope, ContentType: row.ContentType, OccurredAt: row.OccurredAt, Payload: row.Payload})
	hash := intent.Fingerprint()
	_, payloadErr := policyPayloadVersion(row.Payload)
	valid := err == nil && payloadErr == nil && bytes.Equal(row.Fingerprint, hash[:]) && row.Producer == "iam" && row.Destination == topic && row.EventType == eventing.AuthzVersionChanged && row.Scope == "scope:global" && row.SchemaVersion == "v2" && row.ContentType == "application/json"
	if !valid {
		if row.State == "published" {
			r.PublishedContentIssues++
		} else {
			r.InvalidUnfinishedIntents++
		}
	}
}
