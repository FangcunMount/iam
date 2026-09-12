package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	crypto "github.com/FangcunMount/iam/v5/internal/apiserver/infra/crypto"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/accountprovision"
	"io"
	"os"
	"time"
)

func runAccountProvision(args []string, output io.Writer) error {
	if len(args) == 0 || (args[0] != "preflight" && args[0] != "apply") {
		return errors.New("account-provision requires preflight or apply")
	}
	f := flag.NewFlagSet("account-provision", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	var inputPath, reportPath, fingerprint string
	f.StringVar(&inputPath, "input", "", "private account input JSON, including generated password")
	f.StringVar(&reportPath, "report", "", "new private report path; never includes password")
	f.StringVar(&fingerprint, "fingerprint", "", "reviewed preflight fingerprint required for apply")
	if err := f.Parse(args[1:]); err != nil || f.NArg() != 0 {
		return errors.New("invalid account provision arguments")
	}
	input, err := readAccountInput(inputPath)
	if err != nil {
		return err
	}
	reportFile, err := openScopeReport(reportPath)
	if err != nil {
		return err
	}
	defer func() { _ = reportFile.Close() }()
	db, err := roleDatabase("IAM_APISERVER_")
	if err != nil {
		return err
	}
	pool, err := db.DB()
	if err != nil {
		return errors.New("IAM database unavailable")
	}
	defer func() { _ = pool.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// This is the same default hasher configured by the current Authn composition root.
	hasher := crypto.NewArgon2Hasher("")
	var report accountprovision.Report
	if args[0] == "preflight" {
		report, err = accountprovision.Preflight(ctx, db, input, hasher)
	} else {
		report, err = accountprovision.Apply(ctx, db, input, hasher, fingerprint)
	}
	evidence := struct {
		Report accountprovision.Report `json:"report"`
		Error  string                  `json:"error,omitempty"`
	}{Report: report}
	if err != nil {
		evidence.Error = err.Error()
		evidence.Report.State = "unconfirmed"
	}
	if e := json.NewEncoder(reportFile).Encode(evidence); e != nil {
		return errors.New("cannot save private account report; inspect identity before retry")
	}
	if e := reportFile.Sync(); e != nil {
		return e
	}
	if e := reportFile.Close(); e != nil {
		return e
	}
	if _, e := fmt.Fprintf(output, "account provisioning state=%s; details saved privately\n", evidence.Report.State); e != nil {
		return e
	}
	if err != nil {
		return errors.New("account provisioning stopped; inspect restricted report")
	}
	return nil
}
func readAccountInput(path string) (accountprovision.Input, error) {
	var input accountprovision.Input
	file, err := os.Open(path)
	if err != nil {
		return input, errors.New("private account input file required")
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 65536 {
		return input, errors.New("account input must be a private regular file no larger than 64 KiB")
	}
	decoder := json.NewDecoder(io.LimitReader(file, 65537))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&input); err != nil {
		return input, errors.New("invalid account input JSON")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return input, errors.New("exactly one account input required")
	}
	return input, input.Validate()
}
