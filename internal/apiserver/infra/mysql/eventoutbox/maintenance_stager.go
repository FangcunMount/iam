package eventoutbox

import (
	"context"
	"errors"

	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"gorm.io/gorm"
)

// NewMaintenanceStager stages maintenance notifications in the host transaction
// through the SDK standard Outbox only. An explicit mode prevents stale runbooks
// from choosing a writer silently. Schema/drain checks do not prove live writer
// exclusion; operations must still freeze writers during handoff.
func NewMaintenanceStager(ctx context.Context, db *gorm.DB, catalog *eventcatalog.Catalog, mode string) (event.Stager, error) {
	if mode != "standard" {
		return nil, errors.New("explicit --outbox-mode=standard required; legacy maintenance writer is retired (use the reviewed prior release for legacy rollback)")
	}
	if err := CheckReliableSchema(ctx, db); err != nil {
		return nil, err
	}
	if err := CheckLegacyDrained(ctx, db); err != nil {
		return nil, err
	}
	return NewStandardStager(catalog)
}
