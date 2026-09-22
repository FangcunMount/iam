package eventoutbox

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/FangcunMount/reliable-messaging/outbox"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReliableStore adapts the historical table. Schema installation and exclusive
// handoff from the legacy ID-only writer are explicit host prerequisites.
// It borrows DB and never installs schema or starts work in its constructor.
type ReliableStore struct {
	db          *gorm.DB
	topic       string
	legacyStale time.Duration
	clockOffset time.Duration
}

var _ outbox.Store = (*ReliableStore)(nil)

func NewReliableStore(db *gorm.DB, topic string, legacyStale, clockOffset time.Duration) (*ReliableStore, error) {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "mysql" || topic == "" || legacyStale <= 0 || legacyStale > 24*time.Hour || !validHistoricalClock(clockOffset) {
		return nil, errors.New("MySQL, policy topic and bounded legacy lease required")
	}
	return &ReliableStore{db: db, topic: topic, legacyStale: legacyStale, clockOffset: clockOffset}, nil
}

type reliableRow struct {
	OutboxPO     `gorm:"embedded"`
	ClaimToken   *string    `gorm:"column:rm_claim_token"`
	ClaimVersion uint64     `gorm:"column:rm_claim_version"`
	ClaimCount   uint64     `gorm:"column:rm_claim_count"`
	LeaseUntil   *time.Time `gorm:"column:rm_lease_until"`
	Fingerprint  []byte     `gorm:"column:rm_fingerprint"`
}

func (reliableRow) TableName() string { return "domain_event_outbox" }

func (s *ReliableStore) ClaimDue(ctx context.Context, limit int, lease time.Duration) ([]outbox.Claim, error) {
	if limit < 1 || limit > 1000 || lease <= 0 || lease > 24*time.Hour {
		return nil, errors.New("invalid claim bounds")
	}
	claims := make([]outbox.Claim, 0, limit)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []reliableRow
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where(`(status IN ('pending','failed') AND next_attempt_at<=TIMESTAMPADD(MICROSECOND,?,UTC_TIMESTAMP(6))) OR
   (status='publishing' AND rm_claim_token IS NOT NULL AND rm_lease_until<=UTC_TIMESTAMP(6)) OR
   (status='publishing' AND rm_claim_token IS NULL AND updated_at<=TIMESTAMPADD(MICROSECOND,?,UTC_TIMESTAMP(6)))`, int64(s.clockOffset/time.Microsecond), int64(s.clockOffset/time.Microsecond)-ceilMicros(s.legacyStale)).Order("created_at ASC, id ASC").Limit(limit).Find(&rows).Error
		if err != nil {
			return err
		}
		for _, row := range rows {
			intent, e := policyIntent(row.OutboxPO, s.topic)
			fingerprint := intent.Fingerprint()
			code := "unsupported_historical_intent"
			if e == nil && row.Fingerprint != nil && !bytes.Equal(row.Fingerprint, fingerprint[:]) {
				e = outbox.ErrConflict
				code = "historical_identity_conflict"
			}
			if e != nil {
				// Preserve evidence and remove this malformed row from automatic attempts.
				if e = tx.Model(&OutboxPO{}).Where("id=?", row.ID).Updates(map[string]any{"status": "quarantined", "last_error": code, "rm_claim_token": nil, "rm_lease_until": nil, "updated_at": historicalClockNow(s.clockOffset)}).Error; e != nil {
					return e
				}
				continue
			}
			var random [16]byte
			if _, e = rand.Read(random[:]); e != nil {
				return e
			}
			token := hex.EncodeToString(random[:])
			result := tx.Model(&OutboxPO{}).Where("id=?", row.ID).Updates(map[string]any{
				"status": "publishing", "rm_claim_token": token, "rm_fingerprint": fingerprint[:], "rm_claim_version": gorm.Expr("rm_claim_version+1"), "rm_claim_count": gorm.Expr("rm_claim_count+1"),
				"rm_lease_until": gorm.Expr("TIMESTAMPADD(MICROSECOND,?,UTC_TIMESTAMP(6))", ceilMicros(lease)), "updated_at": historicalClockNow(s.clockOffset),
			})
			if result.Error != nil {
				return result.Error
			}
			var claimed reliableRow
			if e = tx.Where("id=?", row.ID).First(&claimed).Error; e != nil {
				return e
			}
			if claimed.LeaseUntil == nil {
				return errors.New("claim lease not persisted")
			}
			claims = append(claims, outbox.Claim{RecordID: strconv.FormatUint(row.ID, 10), Token: token, Version: claimed.ClaimVersion, Attempts: claimed.ClaimCount, LeaseUntil: *claimed.LeaseUntil, Message: intent})
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func (s *ReliableStore) Confirm(ctx context.Context, c outbox.Claim) error {
	return s.mutate(ctx, c, map[string]any{"status": "published", "published_at": historicalClockNow(s.clockOffset)})
}
func (s *ReliableStore) Retry(ctx context.Context, c outbox.Claim, delay time.Duration, code string) error {
	if delay <= 0 {
		return errors.New("positive retry delay required")
	}
	return s.mutate(ctx, c, map[string]any{"status": "failed", "attempt_count": gorm.Expr("attempt_count+1"), "next_attempt_at": gorm.Expr("TIMESTAMPADD(MICROSECOND,?,UTC_TIMESTAMP(6))", int64(s.clockOffset/time.Microsecond)+ceilMicros(delay)+999), "last_error": code})
}
func (s *ReliableStore) Quarantine(ctx context.Context, c outbox.Claim, code string) error {
	return s.mutate(ctx, c, map[string]any{"status": "quarantined", "last_error": code})
}
func (s *ReliableStore) mutate(ctx context.Context, c outbox.Claim, fields map[string]any) error {
	id, err := strconv.ParseUint(c.RecordID, 10, 64)
	if err != nil || id == 0 || strconv.FormatUint(id, 10) != c.RecordID || c.Token == "" || c.Version == 0 {
		return outbox.ErrStaleClaim
	}
	fields["rm_claim_token"] = nil
	fields["rm_lease_until"] = nil
	fields["rm_claim_version"] = gorm.Expr("rm_claim_version+1")
	fields["updated_at"] = historicalClockNow(s.clockOffset)
	result := s.db.WithContext(ctx).Model(&OutboxPO{}).Where("id=? AND status='publishing' AND rm_claim_token=? AND rm_claim_version=? AND rm_lease_until>UTC_TIMESTAMP(6)", id, c.Token, c.Version).Updates(fields)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return outbox.ErrStaleClaim
	}
	return nil
}
func ceilMicros(d time.Duration) int64 {
	n := int64(d / time.Microsecond)
	if d%time.Microsecond != 0 {
		n++
	}
	return n
}
