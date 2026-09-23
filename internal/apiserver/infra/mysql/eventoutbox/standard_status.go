package eventoutbox

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/FangcunMount/iam/v5/internal/pkg/timezone"
	outboxport "github.com/FangcunMount/iam/v5/pkg/outbox"
	"gorm.io/gorm"
)

var ErrUnsafeMessagingHandoff = errors.New("message handoff would leave unfinished records without their owner")

// CheckLegacyDrained is a data gate, not proof that old writers have stopped.
func CheckLegacyDrained(ctx context.Context, db *gorm.DB) error {
	var found []uint64
	if err := db.WithContext(ctx).Raw("SELECT id FROM domain_event_outbox WHERE BINARY status <> 'published' LIMIT 1").Scan(&found).Error; err != nil {
		return err
	}
	if len(found) != 0 {
		return ErrUnsafeMessagingHandoff
	}
	return nil
}

// CheckStandardDrained prevents disabling SDK recovery while it still owns work.
// Pre-M3 databases have no standard table and retain their legacy startup path.
func CheckStandardDrained(ctx context.Context, db *gorm.DB) error {
	tables, err := db.WithContext(ctx).Migrator().GetTables()
	if err != nil {
		return errors.Join(ErrUnsafeMessagingHandoff, err)
	}
	if !slices.Contains(tables, "rm_outbox") {
		return nil
	}
	var found []uint64
	if err := db.WithContext(ctx).Raw("SELECT id FROM rm_outbox WHERE BINARY state <> 'published' LIMIT 1").Scan(&found).Error; err != nil {
		return errors.Join(ErrUnsafeMessagingHandoff, err)
	}
	if len(found) != 0 {
		return ErrUnsafeMessagingHandoff
	}
	return nil
}

type StandardStatusReader struct{ db *gorm.DB }

func NewStandardStatusReader(db *gorm.DB) *StandardStatusReader {
	return &StandardStatusReader{db: db}
}

var _ outboxport.StatusReader = (*StandardStatusReader)(nil)

func (s *StandardStatusReader) OutboxStatusSnapshot(ctx context.Context, now time.Time) (outboxport.StatusSnapshot, error) {
	if now.IsZero() {
		now = time.Now()
	}
	result := outboxport.StatusSnapshot{Store: "iam-standard-and-legacy-outbox", GeneratedAt: now.In(timezone.Location)}
	if s == nil || s.db == nil {
		return result, errors.New("outbox database unavailable")
	}
	var groups []struct {
		Status string
		Count  int64
		Oldest string
	}
	// Explicit text decoding keeps standard UTC dates independent of driver loc.
	// Group unknown states too, rather than silently omitting future/corrupt work.
	err := s.db.WithContext(ctx).Raw(`SELECT
 CASE WHEN BINARY state IN ('pending','retry_wait','publishing','quarantined') THEN CONCAT('standard_',state) ELSE 'standard_unknown' END AS status,
 COUNT(*) AS count, DATE_FORMAT(MIN(created_at),'%Y-%m-%d %H:%i:%s.%f') AS oldest
 FROM rm_outbox WHERE BINARY state <> 'published' GROUP BY 1
 UNION ALL SELECT
 CASE WHEN BINARY status IN ('pending','failed','publishing','quarantined') THEN CONCAT('legacy_',status) ELSE 'legacy_unknown' END AS status,
 COUNT(*) AS count, DATE_FORMAT(MIN(created_at),'%Y-%m-%d %H:%i:%s.%f') AS oldest
 FROM domain_event_outbox WHERE BINARY status <> 'published' GROUP BY 1`).Scan(&groups).Error
	if err != nil {
		return result, err
	}
	for _, group := range groups {
		location := time.UTC
		if strings.HasPrefix(group.Status, "legacy_") {
			location = timezone.Location
		}
		oldest, err := time.ParseInLocation("2006-01-02 15:04:05.000000", group.Oldest, location)
		if err != nil {
			return result, err
		}
		oldest = oldest.In(timezone.Location)
		result.Buckets = append(result.Buckets, outboxport.StatusBucket{Status: group.Status, Count: group.Count, OldestCreatedAt: &oldest, OldestAgeSeconds: max(0, now.Sub(oldest).Seconds())})
	}
	return result, nil
}
