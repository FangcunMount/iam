package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/freshbootstrap"
	"github.com/FangcunMount/iam/v5/internal/pkg/migration"
	"github.com/google/uuid"
)

const freshBootstrapConfirmation = "CLAIM_FRESH_IAM_DATABASE"

func runFreshTransactionBootstrap(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("fresh-transaction-bootstrap", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	database := flags.String("database", "", "reviewed empty MySQL database")
	server := flags.String("server-uuid", "", "reviewed MySQL server UUID")
	confirm := flags.String("confirm", "", "required offline bootstrap confirmation")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return errors.New("invalid fresh transaction bootstrap arguments")
	}
	serverUUID, err := uuid.Parse(*server)
	if *database == "" || err != nil || serverUUID == uuid.Nil || serverUUID.String() != *server || *confirm != freshBootstrapConfirmation {
		return errors.New("fresh transaction bootstrap requires --database, canonical --server-uuid and --confirm")
	}
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("MYSQL_TLS_MODE")), "VERIFY_IDENTITY") {
		return errors.New("fresh transaction bootstrap requires MYSQL_TLS_MODE=VERIFY_IDENTITY")
	}
	db, err := authzConvergeDatabaseFromEnvironment()
	if err != nil {
		return err
	}
	pool, err := db.DB()
	if err != nil {
		return errors.New("fresh transaction bootstrap database pool unavailable")
	}
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	migrator := migration.NewMigrator(pool, &migration.Config{Enabled: true, Database: *database})
	version, changed, err := migrator.RunFreshForTransactionDelivery(ctx, *server, func(conn *sql.Conn) (migration.FreshBootstrapHooks, error) {
		return freshbootstrap.Bind(ctx, conn, db, authzV3ConvergeCatalog, true)
	})
	if err != nil {
		return errors.New("fresh transaction bootstrap refused or interrupted; inspect the reviewed database identity and migration journal")
	}
	return writeJSON(output, struct {
		Database   string `json:"database"`
		ServerUUID string `json:"server_uuid"`
		Version    uint   `json:"version"`
		Changed    bool   `json:"changed"`
	}{*database, *server, version, changed})
}
