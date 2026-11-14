package freshbootstrap

import (
	"context"
	"database/sql"

	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/bootstrapdelivery"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/conditionretire"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/rolemodel"
	"github.com/FangcunMount/iam/v5/internal/pkg/migration"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Bind(ctx context.Context, conn *sql.Conn, original *gorm.DB, catalogPath string, transactional bool) (migration.FreshBootstrapHooks, error) {
	bound, err := gorm.Open(gormmysql.New(gormmysql.Config{Conn: conn}), &gorm.Config{
		Logger: original.Logger, NamingStrategy: original.NamingStrategy,
	})
	if err != nil {
		return migration.FreshBootstrapHooks{}, err
	}
	catalog, err := eventcatalog.Load(catalogPath)
	if err != nil {
		return migration.FreshBootstrapHooks{}, err
	}
	events := eventcatalog.NewCatalog(catalog)
	hooks := migration.FreshBootstrapHooks{
		PrepareTenant: func(checkpoint func(*sql.Tx) error) error {
			return rolemodel.PrepareBootstrapTenantWithCheckpoint(ctx, bound, checkpoint)
		},
	}
	if transactional {
		notifications, err := bootstrapdelivery.Prepare(ctx, conn, events)
		if err != nil {
			return migration.FreshBootstrapHooks{}, err
		}
		hooks.IndependentRoles = func() error { return rolemodel.BootstrapIndependentRoles(ctx, bound, notifications.Stage(33)) }
		hooks.RetireConditions = func() error { return conditionretire.Bootstrap(ctx, bound, notifications.Stage(35)) }
		hooks.VerifyNotification = func(phase uint) error { return notifications.Verify(ctx, phase) }
		return hooks, nil
	}
	stager, err := eventoutbox.NewBootstrapStager(events)
	if err != nil {
		return migration.FreshBootstrapHooks{}, err
	}
	hooks.IndependentRoles = func() error { return rolemodel.BootstrapIndependentRoles(ctx, bound, stager) }
	hooks.RetireConditions = func() error { return conditionretire.Bootstrap(ctx, bound, stager) }
	return hooks, nil
}
