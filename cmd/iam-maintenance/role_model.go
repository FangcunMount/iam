package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"github.com/FangcunMount/iam/v5/internal/pkg/timezone"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/infra/mysql/eventoutbox"
	"github.com/FangcunMount/iam/v5/internal/apiserver/maintenance/rolemodel"
	"github.com/FangcunMount/iam/v5/pkg/eventcatalog"
	drivermysql "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func runRoleModelMigration(args []string, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("role-model-migrate requires status, preflight, apply, verify, rollback, or archive-inheritance")
	}
	mode := args[0]
	if mode != "status" && mode != "preflight" && mode != "apply" && mode != "verify" && mode != "rollback" && mode != "archive-inheritance" {
		return errors.New("invalid role migration operation")
	}
	f := flag.NewFlagSet("role-model-migrate", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	fingerprint := f.String("fingerprint", "", "reviewed source fingerprint")
	stopped := f.Bool("writes-stopped", false, "authorization and relevant identity writers are stopped")
	catalogPath := f.String("event-catalog", "configs/events.yaml", "durable event catalog")
	timeout := f.Duration("timeout", time.Minute, "operation timeout")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 || *timeout <= 0 {
		return errors.New("invalid role migration arguments")
	}
	if (mode == "apply" || mode == "rollback" || mode == "archive-inheritance") && (!*stopped || len(*fingerprint) != 64) {
		return errors.New("role migration mutation requires --writes-stopped and --fingerprint")
	}
	iam, err := roleDatabase("")
	if err != nil {
		return err
	}
	pool, err := iam.DB()
	if err != nil {
		return err
	}
	defer func() { _ = pool.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	status, err := rolemodel.Status(ctx, iam)
	if mode == "status" || err != nil {
		if outputErr := writeJSON(output, status); outputErr != nil {
			return outputErr
		}
		return err
	}
	var qs *gorm.DB
	if (mode == "preflight" || mode == "apply") && status.State == "pending" {
		qs, err = roleDatabase("QS_")
		if err != nil {
			return err
		}
		qpool, err := qs.DB()
		if err != nil {
			return err
		}
		defer func() { _ = qpool.Close() }()
	}
	switch mode {
	case "preflight":
		if status.State != "pending" && status.State != "applied_unchanged" {
			if err := writeJSON(output, status); err != nil {
				return err
			}
			return errors.New("migration is not executable: " + status.State)
		}
		p, err := rolemodel.Preflight(ctx, iam, qs)
		if err != nil {
			return err
		}
		if status.State == "pending" {
			status.Fingerprint = p.Fingerprint
			status.NextAction = "apply"
		}
		if err = writeRoleReport(output, p, status); err != nil {
			return err
		}
		return p.Validate()
	case "archive-inheritance":
		a, err := rolemodel.ArchiveInheritance(ctx, iam, *fingerprint, *stopped)
		if err != nil {
			return err
		}
		return writeRoleReport(output, a, status)
	case "verify":
		r, err := rolemodel.Verify(ctx, iam)
		if err != nil {
			return err
		}
		return writeRoleReport(output, r, status)
	default:
		cfg, err := eventcatalog.Load(*catalogPath)
		if err != nil {
			return err
		}
		stager := eventoutbox.NewStore(iam, eventcatalog.NewCatalog(cfg))
		var receipt *rolemodel.Receipt
		if mode == "apply" {
			receipt, err = rolemodel.Apply(ctx, iam, qs, stager, *fingerprint, *stopped)
		} else {
			receipt, err = rolemodel.Rollback(ctx, iam, stager, *fingerprint, *stopped)
		}
		if err != nil {
			return err
		}
		status, err = rolemodel.Status(ctx, iam)
		if err != nil {
			return err
		}
		return writeRoleReport(output, receipt, status)
	}
}

func writeRoleReport(output io.Writer, value any, status rolemodel.StatusReport) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var result map[string]any
	if err = json.Unmarshal(data, &result); err != nil {
		return err
	}
	data, err = json.Marshal(status)
	if err != nil {
		return err
	}
	var fields map[string]any
	if err = json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for k, v := range fields {
		result[k] = v
	}
	if status.State == "applied_unchanged" {
		result["ready"] = false
	}
	return writeJSON(output, result)
}

// Credentials remain in process environment. Driver Config escapes special
// characters correctly and connection failures never print a DSN.
func roleDatabase(prefix string) (*gorm.DB, error) {
	get := func(key string) string { return strings.TrimSpace(os.Getenv(prefix + "MYSQL_" + key)) }
	host := get("HOST")
	port := get("PORT")
	if port == "" {
		port = "3306"
	}
	if h, p, err := net.SplitHostPort(host); err == nil {
		host, port = h, p
	}
	user := get("USERNAME")
	if user == "" {
		user = get("USER")
	}
	database := get("DATABASE")
	if database == "" {
		database = get("DBNAME")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 || host == "" || user == "" || database == "" {
		return nil, errors.New(prefix + "MYSQL connection environment is invalid")
	}
	cfg := drivermysql.NewConfig()
	cfg.User = user
	cfg.Passwd = os.Getenv(prefix + "MYSQL_PASSWORD")
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(host, port)
	cfg.DBName = database
	cfg.ParseTime = true
	cfg.Loc = timezone.Location
	cfg.Params = map[string]string{"time_zone": "'+08:00'"}
	cfg.Timeout = 8 * time.Second
	cfg.ReadTimeout = time.Minute
	cfg.WriteTimeout = time.Minute
	db, err := gorm.Open(gormmysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, errors.New(prefix + "MYSQL connection failed")
	}
	return db, nil
}
