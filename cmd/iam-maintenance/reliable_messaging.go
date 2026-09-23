package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/eventing"
	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
)

func runReliableMessaging(args []string, output io.Writer) error {
	if len(args) == 0 || args[0] != "preflight" {
		return errors.New("reliable-messaging supports only read-only preflight")
	}
	f := flag.NewFlagSet("reliable-messaging preflight", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	target := f.String("target", "sdk", "data-readiness target: sdk, legacy or schema-down")
	catalogPath := f.String("event-catalog", "configs/events.yaml", "host event catalog")
	maxRows := f.Int("max-rows", 10000, "maximum inspected rows, 1..1000000")
	timeout := f.Duration("timeout", 30*time.Second, "snapshot timeout, at most 5m")
	if err := f.Parse(args[1:]); err != nil || f.NArg() != 0 {
		return errors.New("invalid reliable-messaging preflight arguments")
	}
	if (*target != "sdk" && *target != "legacy" && *target != "schema-down") || *maxRows < 1 || *maxRows > 1_000_000 || *timeout <= 0 || *timeout > 5*time.Minute {
		return errors.New("invalid preflight target or bounds")
	}
	cfg, err := eventcatalog.Load(*catalogPath)
	if err != nil {
		return errors.New("preflight event catalog unavailable")
	}
	catalog := eventcatalog.NewCatalog(cfg)
	topic, ok := catalog.GetTopicForEvent(eventing.AuthzVersionChanged)
	if !ok || !catalog.IsDurableOutbox(eventing.AuthzVersionChanged) {
		return errors.New("preflight requires the durable policy route")
	}
	db, err := roleDatabase("IAM_APISERVER_")
	if err != nil {
		return err
	}
	pool, err := db.DB()
	if err != nil {
		return errors.New("preflight database unavailable")
	}
	defer func() { _ = pool.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	report, err := eventoutbox.InspectReliableOutbox(ctx, db, topic, *maxRows)
	if err != nil {
		return errors.New("preflight could not complete; verify schema 38, connection and snapshot budget")
	}
	if err := writeJSON(output, report); err != nil {
		return err
	}
	ready := report.SDKDataReady
	if *target == "legacy" {
		ready = report.LegacyRollbackDataReady
	} else if *target == "schema-down" {
		ready = report.UnusedSchemaCanBeRemoved
	}
	if !ready {
		return errors.New("preflight data conditions not met; inspect counts, no changes applied")
	}
	return nil // Data check only. JSON always leaves writer exclusion and authorization false.
}
