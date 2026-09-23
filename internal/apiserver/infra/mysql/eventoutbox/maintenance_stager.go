package eventoutbox

import (
	"context"
	"errors"
	"slices"

	"github.com/FangcunMount/iam/v5/pkg/event"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	"gorm.io/gorm"
)

// NewMaintenanceStager selects an insert-only writer for a reviewed maintenance
// operation. Schema presence never selects SDK mode. These checks cannot prove
// the live Relay mode or exclude another process: operators must freeze writers
// during handoff and choose the mode owned by the reviewed running release.
func NewMaintenanceStager(ctx context.Context, db *gorm.DB, catalog *eventcatalog.Catalog, mode string) (event.Stager, error) {
	if mode != "" && mode != "standard" && mode != "legacy" {
		return nil, errors.New("outbox-mode must be standard or legacy")
	}
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "mysql" {
		return nil, errors.New("maintenance Outbox requires MySQL")
	}
	var versions []struct {
		Version uint64
		Dirty   bool
	}
	if err := db.WithContext(ctx).Raw("SELECT version, dirty FROM schema_migrations").Scan(&versions).Error; err != nil {
		return nil, err
	}
	if len(versions) != 1 || versions[0].Dirty {
		return nil, errors.New("maintenance Outbox requires a clean migration journal")
	}
	if mode == "" {
		tables, err := db.WithContext(ctx).Migrator().GetTables()
		if err != nil {
			return nil, err
		}
		if versions[0].Version >= 38 || slices.Contains(tables, "rm_outbox") {
			return nil, errors.New("explicit --outbox-mode=standard or legacy matching the reviewed Relay is required after standard schema installation")
		}
		mode = "legacy" // Preserve pre-M3 maintenance on its original schema.
	}
	// A migrated database must retain its standard receipts even in legacy mode.
	if mode == "standard" || versions[0].Version >= 38 {
		if err := CheckReliableSchema(ctx, db); err != nil {
			return nil, err
		}
	}
	if mode == "standard" {
		if err := CheckLegacyDrained(ctx, db); err != nil {
			return nil, err
		}
		return NewStandardStager(catalog)
	}
	if err := CheckStandardDrained(ctx, db); err != nil {
		return nil, err
	}
	return NewStore(db, catalog), nil
}
