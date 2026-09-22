package eventoutbox

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Original DATETIME columns retain the historical writer's wall-clock convention.
// Lease columns are always UTC. Offset must be audited before forward/backward
// handoff; it cannot be inferred from a DATETIME value or a message fingerprint.
func validHistoricalClock(offset time.Duration) bool {
	return offset >= -14*time.Hour && offset <= 14*time.Hour && offset%time.Minute == 0
}

func historicalClockNow(offset time.Duration) clause.Expr {
	return gorm.Expr("TIMESTAMPADD(MICROSECOND,?,UTC_TIMESTAMP(3))", int64(offset/time.Microsecond))
}
