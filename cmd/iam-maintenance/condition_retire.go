package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/conditionretire"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
)

func runConditionRetirement(args []string, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("condition-authz-retire requires status, preflight, apply, verify, or rollback")
	}
	mode := args[0]
	switch mode {
	case "status", "preflight", "apply", "verify", "rollback":
	default:
		return errors.New("invalid condition retirement operation")
	}
	f := flag.NewFlagSet("condition-authz-retire", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	fp := f.String("fingerprint", "", "reviewed preflight fingerprint")
	stopped := f.Bool("writes-stopped", false, "authorization writers and affected operations are stopped")
	catalog := f.String("event-catalog", "configs/events.yaml", "durable event catalog")
	outboxMode := f.String("outbox-mode", "", "standard or legacy, matching the reviewed running Relay; required after migration 38")
	timeout := f.Duration("timeout", time.Minute, "operation timeout")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 || *timeout <= 0 {
		return errors.New("invalid retirement arguments")
	}
	if (mode == "apply" || mode == "rollback") && (!*stopped || len(*fp) != 64) {
		return errors.New("mutation requires --writes-stopped and --fingerprint")
	}
	db, err := roleDatabase("")
	if err != nil {
		return err
	}
	pool, err := db.DB()
	if err != nil {
		return err
	}
	defer func() { _ = pool.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	var report conditionretire.Report
	switch mode {
	case "status":
		report, err = conditionretire.Status(ctx, db)
	case "preflight":
		report, err = conditionretire.Preflight(ctx, db)
	case "verify":
		report, err = conditionretire.Verify(ctx, db)
	default:
		cfg, e := eventcatalog.Load(*catalog)
		if e != nil {
			return e
		}
		stager, gateErr := eventoutbox.NewMaintenanceStager(ctx, db, eventcatalog.NewCatalog(cfg), *outboxMode)
		if gateErr != nil {
			return gateErr
		}
		if mode == "apply" {
			_, err = conditionretire.Apply(ctx, db, stager, *fp, *stopped)
		} else {
			_, err = conditionretire.Rollback(ctx, db, stager, *fp, *stopped)
		}
		if err == nil {
			report, err = conditionretire.Status(ctx, db)
		}
	}
	if e := writeJSON(output, report); e != nil {
		return e
	}
	return err
}
