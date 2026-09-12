package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/scopemigrate"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
)

type scopeOptions struct {
	mode, id, actor, fingerprint, mapping, report, catalog string
	stopped                                                bool
	timeout                                                time.Duration
}

func parseScopeOptions(args []string) (scopeOptions, error) {
	var o scopeOptions
	if len(args) == 0 {
		return o, errors.New("scope-migrate requires status, preflight, apply, verify, or rollback")
	}
	o.mode = args[0]
	switch o.mode {
	case "status", "preflight", "apply", "verify", "rollback":
	default:
		return o, errors.New("invalid Scope operation")
	}
	f := flag.NewFlagSet("scope-migrate", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	f.StringVar(&o.id, "migration-id", "", "independent migration identifier")
	f.StringVar(&o.actor, "actor-id", "", "audited IAM user ID")
	f.StringVar(&o.fingerprint, "fingerprint", "", "reviewed fact fingerprint")
	f.StringVar(&o.mapping, "mapping", "", "explicit version 1 assignment mapping JSON")
	f.StringVar(&o.report, "report", "", "new absolute report path in private directory")
	f.StringVar(&o.catalog, "event-catalog", "configs/events.yaml", "durable event catalog")
	f.BoolVar(&o.stopped, "writes-stopped", false, "IAM authorization and QS membership/store writers paused")
	f.DurationVar(&o.timeout, "timeout", 2*time.Minute, "overall operation timeout")
	if err := f.Parse(args[1:]); err != nil {
		return o, errors.New("invalid Scope command arguments")
	}
	if f.NArg() != 0 || o.timeout <= 0 || o.id == "" || len(o.id) > 64 || strings.TrimSpace(o.id) != o.id || !filepath.IsAbs(o.report) {
		return o, errors.New("migration-id, absolute report path and positive timeout required")
	}
	if o.mode == "apply" || o.mode == "rollback" {
		actor, e := meta.ParseID(o.actor)
		hash, herr := hex.DecodeString(o.fingerprint)
		if !o.stopped || e != nil || actor <= 0 || actor.String() != o.actor || herr != nil || len(hash) != 32 {
			return o, errors.New("mutation requires writes-stopped, canonical actor-id and reviewed fingerprint")
		}
	}
	return o, nil
}
func openScopeReport(path string) (*os.File, error) {
	parent, err := os.Stat(filepath.Dir(path))
	if err != nil || !parent.IsDir() || parent.Mode().Perm()&0077 != 0 {
		return nil, errors.New("report directory must exist and be private (0700)")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, errors.New("report must be a new writable file; existing reports are preserved")
	}
	return file, nil
}
func runScopeMigration(args []string, output io.Writer) error {
	return runScopeMigrationWith(args, output, executeScopeMigration)
}
func runScopeMigrationWith(args []string, output io.Writer, execute func(scopeOptions) (scopemigrate.Report, error)) error {
	o, err := parseScopeOptions(args)
	if err != nil {
		return err
	}
	reportFile, err := openScopeReport(o.report)
	if err != nil {
		return err
	}
	defer func() { _ = reportFile.Close() }()
	report, runErr := execute(o)
	evidence := struct {
		Report scopemigrate.Report `json:"report"`
		Error  string              `json:"error,omitempty"`
	}{Report: report}
	if runErr != nil {
		evidence.Error = runErr.Error()
	}
	if err = json.NewEncoder(reportFile).Encode(evidence); err != nil {
		return errors.New("failed to save restricted Scope report; inspect migration receipt before retry")
	}
	if err = reportFile.Sync(); err != nil {
		return errors.New("failed to sync Scope report; inspect migration receipt before retry")
	}
	if err = reportFile.Close(); err != nil {
		return errors.New("failed to close Scope report; inspect migration receipt before retry")
	}
	// User IDs, mappings and detailed errors stay in the restricted report.
	summary := struct {
		State       string `json:"state"`
		NextAction  string `json:"next_action"`
		MigrationID string `json:"migration_id"`
		Changes     int    `json:"changes"`
		Issues      int    `json:"issues"`
		Success     bool   `json:"success"`
	}{State: report.State, NextAction: report.NextAction, MigrationID: o.id, Success: runErr == nil}
	if report.Plan != nil {
		summary.Changes = len(report.Plan.Changes)
		summary.Issues = len(report.Plan.Issues)
	}
	if err = writeJSON(output, summary); err != nil {
		return err
	}
	if runErr != nil {
		return errors.New("Scope operation failed; inspect the restricted report")
	}
	return nil
}
func executeScopeMigration(o scopeOptions) (scopemigrate.Report, error) {
	fail := scopemigrate.Report{State: "invalid", NextAction: "stop", MigrationID: o.id}
	iam, err := roleDatabase("")
	if err != nil {
		return fail, err
	}
	pool, err := iam.DB()
	if err != nil {
		return fail, err
	}
	defer func() { _ = pool.Close() }()
	qs, err := roleDatabase("QS_")
	if err != nil {
		return fail, err
	}
	qpool, err := qs.DB()
	if err != nil {
		return fail, err
	}
	defer func() { _ = qpool.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()
	report, err := scopemigrate.Status(ctx, iam, qs, o.id)
	if err != nil || o.mode == "status" {
		return report, err
	}
	var input scopemigrate.Input
	if (o.mode == "preflight" || o.mode == "apply") && report.State == "pending" {
		if o.mapping == "" {
			return report, errors.New("pending migration requires explicit mapping file")
		}
		file, e := os.Open(o.mapping)
		if e != nil {
			return report, e
		}
		defer func() { _ = file.Close() }()
		info, e := file.Stat()
		if e != nil || !info.Mode().IsRegular() || info.Size() > 16*1024*1024 {
			return report, errors.New("mapping must be a regular JSON file no larger than 16 MiB")
		}
		input, e = scopemigrate.Decode(io.LimitReader(file, 16*1024*1024))
		if e != nil {
			return report, fmt.Errorf("invalid mapping: %w", e)
		}
	}
	switch o.mode {
	case "preflight":
		return scopemigrate.Preflight(ctx, iam, qs, o.id, input)
	case "verify":
		return scopemigrate.Verify(ctx, iam, qs, o.id)
	default:
		cfg, e := eventcatalog.Load(o.catalog)
		if e != nil {
			return report, e
		}
		stager := eventoutbox.NewStore(iam, eventcatalog.NewCatalog(cfg))
		if o.mode == "apply" {
			_, err = scopemigrate.Apply(ctx, iam, qs, stager, o.id, o.actor, o.fingerprint, input, o.stopped)
		} else {
			_, err = scopemigrate.Rollback(ctx, iam, qs, stager, o.id, o.actor, o.fingerprint, o.stopped)
		}
		if err != nil {
			return report, err
		}
		return scopemigrate.Status(ctx, iam, qs, o.id)
	}
}
