package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/FangcunMount/iam/v2/internal/apiserver/maintenance"
	mysql "github.com/go-sql-driver/mysql"
)

const applyConfirmation = "BACKFILL_AUTHN_LEGACY_MISSING"

type options struct {
	apply   bool
	confirm string
	timeout time.Duration
}

func main() {
	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	dsn, err := dsnFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open reconciliation database failed")
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "connect reconciliation database failed")
		os.Exit(1)
	}
	summary, reconcileErr := maintenance.ReconcileAuthNLegacy(ctx, db, maintenance.AuthNLegacyReconcileOptions{
		Apply: opts.apply,
	})
	encoded, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(encoded))
	if reconcileErr != nil {
		if errors.Is(reconcileErr, maintenance.ErrAuthNLegacyConflicts) {
			fmt.Fprintln(os.Stderr, "AuthN legacy reconciliation is blocked by hard conflicts")
		} else {
			fmt.Fprintln(os.Stderr, "AuthN legacy reconciliation failed")
		}
		os.Exit(1)
	}
}

func parseOptions(args []string) (options, error) {
	flags := flag.NewFlagSet("backfill_authn_loginidentity", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var opts options
	flags.BoolVar(&opts.apply, "apply", false, "insert missing canonical AuthN facts; default is dry-run")
	flags.StringVar(&opts.confirm, "confirm", "", "required confirmation phrase for apply")
	flags.DurationVar(&opts.timeout, "timeout", 5*time.Minute, "overall timeout")
	if err := flags.Parse(args); err != nil {
		return options{}, fmt.Errorf("invalid reconciliation arguments")
	}
	if opts.timeout <= 0 {
		return options{}, fmt.Errorf("timeout must be positive")
	}
	if opts.apply && opts.confirm != applyConfirmation {
		return options{}, fmt.Errorf("apply requires the AuthN legacy confirmation phrase")
	}
	if !opts.apply && strings.TrimSpace(opts.confirm) != "" {
		return options{}, fmt.Errorf("confirm is only accepted together with apply")
	}
	return opts, nil
}

func dsnFromEnv() (string, error) {
	if dsn := strings.TrimSpace(firstEnv("IAM_MYSQL_DSN", "MYSQL_DSN")); dsn != "" {
		config, err := mysql.ParseDSN(dsn)
		if err != nil {
			return "", fmt.Errorf("MySQL reconciliation DSN is invalid")
		}
		config.ParseTime = true
		config.Loc = time.Local
		if config.Collation == "" {
			config.Collation = "utf8mb4_unicode_ci"
		}
		return config.FormatDSN(), nil
	}
	host := strings.TrimSpace(firstEnv("IAM_APISERVER_MYSQL_HOST", "MYSQL_HOST"))
	username := firstEnv(
		"IAM_APISERVER_MYSQL_USERNAME",
		"IAM_APISERVER_MYSQL_USER",
		"MYSQL_USERNAME",
		"MYSQL_USER",
	)
	password := firstEnv("IAM_APISERVER_MYSQL_PASSWORD", "MYSQL_PASSWORD", "MYSQL_PASS")
	database := strings.TrimSpace(firstEnv(
		"IAM_APISERVER_MYSQL_DATABASE",
		"IAM_APISERVER_MYSQL_DBNAME",
		"MYSQL_DATABASE",
		"MYSQL_DBNAME",
	))
	if host == "" || strings.TrimSpace(username) == "" || database == "" {
		return "", fmt.Errorf("MySQL reconciliation environment is incomplete")
	}
	if _, _, err := net.SplitHostPort(host); err != nil {
		port := strings.TrimSpace(firstEnv("IAM_APISERVER_MYSQL_PORT", "MYSQL_PORT"))
		if port == "" {
			port = "3306"
		}
		parsedPort, err := strconv.Atoi(port)
		if err != nil || parsedPort < 1 || parsedPort > 65535 {
			return "", fmt.Errorf("MySQL reconciliation port is invalid")
		}
		host = net.JoinHostPort(host, port)
	}
	config := mysql.NewConfig()
	config.User = username
	config.Passwd = password
	config.Net = "tcp"
	config.Addr = host
	config.DBName = database
	config.ParseTime = true
	config.Loc = time.Local
	config.Collation = "utf8mb4_unicode_ci"
	return config.FormatDSN(), nil
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}
