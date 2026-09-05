package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/attributeproviders"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/eventoutbox"
	authzmysqluow "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v3/internal/apiserver/maintenance"
	"github.com/FangcunMount/iam/v3/pkg/eventcatalog"
)

func runAuthzHardening(args []string, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("authz-hardening requires preflight, apply or evidence")
	}
	operation := args[0]
	if operation != "preflight" && operation != "apply" && operation != "evidence" {
		return errors.New("unsupported authz-hardening operation")
	}
	flags := flag.NewFlagSet("authz-hardening "+operation, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	expected := flags.String("expected-fingerprint", "", "reviewed preflight fingerprint")
	confirm := flags.String("confirm", "", "apply confirmation")
	providerPath := flags.String("attribute-providers", "configs/authz_attribute_providers.yaml", "trusted attribute provider file")
	eventsPath := flags.String("events-catalog", authzV3ConvergeCatalog, "event catalog file")
	build := flags.String("build-sha", "", "exact candidate build SHA")
	timeout := flags.Duration("timeout", 10*time.Minute, "operation timeout")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		return errors.New("invalid authz-hardening arguments")
	}
	if *timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	if operation == "apply" {
		if *confirm != maintenance.AuthzHardeningConfirmation || len(strings.TrimSpace(*expected)) != 64 {
			return errors.New("apply requires confirmation and reviewed fingerprint")
		}
	} else if *confirm != "" || *expected != "" {
		return errors.New("confirmation and fingerprint are only accepted for apply")
	}
	if operation == "evidence" && strings.TrimSpace(*build) == "" {
		return errors.New("evidence requires an exact build sha")
	}
	providers, err := attributeproviders.Load(*providerPath)
	if err != nil {
		return errors.New("authorization attribute provider configuration failed")
	}
	db, err := authzConvergeDatabaseFromEnvironment()
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return errors.New("authorization database initialization failed")
	}
	defer sqlDB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	var report *maintenance.AuthzHardeningReport
	if operation == "apply" {
		release, lockErr := acquireAuthzV3ConvergeLock(ctx, sqlDB, 30*time.Second)
		if lockErr != nil {
			return lockErr
		}
		defer release()
		config, loadErr := eventcatalog.Load(*eventsPath)
		if loadErr != nil {
			return errors.New("authorization event catalog load failed")
		}
		stager := eventoutbox.NewStore(db, eventcatalog.NewCatalog(config))
		report, err = maintenance.ApplyAuthzHardening(ctx, db, authzmysqluow.NewUnitOfWork(db, nil, stager), providers, *expected)
	} else {
		report, err = maintenance.AnalyzeAuthzHardening(ctx, db, providers)
	}
	if report != nil {
		if writeErr := writeJSON(output, struct {
			Operation string `json:"operation"`
			BuildSHA  string `json:"build_sha,omitempty"`
			Succeeded bool   `json:"succeeded"`
			*maintenance.AuthzHardeningReport
		}{operation, *build, err == nil, report}); writeErr != nil {
			return writeErr
		}
	}
	if err != nil {
		return errors.New("authorization hardening " + operation + " failed; no changes committed")
	}
	if report == nil || len(report.Blockers) > 0 {
		return errors.New("authorization hardening is blocked")
	}
	if operation == "evidence" && !report.Complete {
		return errors.New("authorization hardening data convergence is incomplete")
	}
	return nil
}
