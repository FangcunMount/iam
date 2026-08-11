package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	redisinfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/cache/redis"
	"github.com/FangcunMount/iam/v2/internal/apiserver/maintenance"
	mysql "github.com/go-sql-driver/mysql"
	goredis "github.com/redis/go-redis/v9"
)

const (
	purgeConfirmation       = "PURGE_REFRESH_TOKENS"
	logDisposalConfirmation = "DELETE_PRE_5_4_IAM_LOGS"
	authNLegacyConfirmation = "BACKFILL_AUTHN_LEGACY_MISSING"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, publicMaintenanceError(err))
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("a maintenance subcommand is required")
	}
	switch args[0] {
	case "purge-refresh-tokens":
		return runPurgeRefreshTokens(args[1:], output)
	case "dispose-sensitive-logs":
		return runDisposeSensitiveLogs(args[1:], output)
	case "reconcile-authn-legacy":
		return runReconcileAuthNLegacy(args[1:], output)
	default:
		return errors.New("unsupported maintenance subcommand")
	}
}

func runReconcileAuthNLegacy(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("reconcile-authn-legacy", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	apply := flags.Bool("apply", false, "insert missing canonical AuthN facts")
	requireEligible := flags.Bool("require-eligible", false, "fail unless legacy AuthN is retirement eligible")
	confirm := flags.String("confirm", "", "required confirmation phrase")
	batchSize := flags.Int("batch-size", maintenance.DefaultAuthNLegacyBatchSize, "maximum canonical rows inserted per apply")
	timeout := flags.Duration("timeout", 5*time.Minute, "overall timeout")
	if err := flags.Parse(args); err != nil {
		return errors.New("invalid reconcile-authn-legacy arguments")
	}
	if *timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	if *batchSize <= 0 || *batchSize > maintenance.MaxAuthNLegacyBatchSize {
		return errors.New("batch-size is outside the allowed AuthN reconciliation range")
	}
	if *apply && *confirm != authNLegacyConfirmation {
		return errors.New("apply requires the AuthN legacy confirmation phrase")
	}
	if *apply && *requireEligible {
		return errors.New("require-eligible is only accepted during dry-run")
	}
	if !*apply && strings.TrimSpace(*confirm) != "" {
		return errors.New("confirm is only accepted together with apply")
	}

	dsn, err := mysqlDSNFromEnvironment()
	if err != nil {
		return err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return errors.New("open AuthN reconciliation database failed")
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return errors.New("connect AuthN reconciliation database failed")
	}
	summary, reconcileErr := maintenance.ReconcileAuthNLegacy(ctx, db, maintenance.AuthNLegacyReconcileOptions{
		Apply:     *apply,
		BatchSize: *batchSize,
	})
	if err := writeJSON(output, summary); err != nil {
		return err
	}
	return authNReconcileExitError(summary, reconcileErr, *requireEligible)
}

func authNReconcileExitError(
	summary maintenance.AuthNLegacyReconcileSummary,
	reconcileErr error,
	requireEligible bool,
) error {
	if reconcileErr != nil {
		if errors.Is(reconcileErr, maintenance.ErrAuthNLegacyConflicts) {
			return errors.New("AuthN legacy reconciliation is blocked by hard conflicts")
		}
		return errors.New("AuthN legacy reconciliation failed")
	}
	if requireEligible && !summary.RetirementEligible {
		return errors.New("AuthN legacy reconciliation is not retirement eligible")
	}
	return nil
}

func mysqlDSNFromEnvironment() (string, error) {
	if dsn := strings.TrimSpace(firstNonEmptyEnvironment("IAM_MYSQL_DSN", "MYSQL_DSN")); dsn != "" {
		config, err := mysql.ParseDSN(dsn)
		if err != nil {
			return "", errors.New("MySQL reconciliation DSN is invalid")
		}
		config.ParseTime = true
		config.Loc = time.Local
		if config.Collation == "" {
			config.Collation = "utf8mb4_unicode_ci"
		}
		return config.FormatDSN(), nil
	}
	host := strings.TrimSpace(firstNonEmptyEnvironment("IAM_APISERVER_MYSQL_HOST", "MYSQL_HOST"))
	username := firstNonEmptyEnvironment(
		"IAM_APISERVER_MYSQL_USERNAME",
		"IAM_APISERVER_MYSQL_USER",
		"MYSQL_USERNAME",
		"MYSQL_USER",
	)
	password := firstNonEmptyEnvironment("IAM_APISERVER_MYSQL_PASSWORD", "MYSQL_PASSWORD", "MYSQL_PASS")
	database := strings.TrimSpace(firstNonEmptyEnvironment(
		"IAM_APISERVER_MYSQL_DATABASE",
		"IAM_APISERVER_MYSQL_DBNAME",
		"MYSQL_DATABASE",
		"MYSQL_DBNAME",
	))
	if host == "" || strings.TrimSpace(username) == "" || database == "" {
		return "", errors.New("MySQL reconciliation environment is incomplete")
	}
	if _, _, err := net.SplitHostPort(host); err != nil {
		port := strings.TrimSpace(firstNonEmptyEnvironment("IAM_APISERVER_MYSQL_PORT", "MYSQL_PORT"))
		if port == "" {
			port = "3306"
		}
		parsedPort, err := strconv.Atoi(port)
		if err != nil || parsedPort < 1 || parsedPort > 65535 {
			return "", errors.New("MySQL reconciliation port is invalid")
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

func firstNonEmptyEnvironment(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func runPurgeRefreshTokens(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("purge-refresh-tokens", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	apply := flags.Bool("apply", false, "apply deletion")
	confirm := flags.String("confirm", "", "required confirmation phrase")
	batchSize := flags.Int64("batch-size", 500, "SCAN/UNLINK batch size")
	timeout := flags.Duration("timeout", 2*time.Minute, "overall timeout")
	if err := flags.Parse(args); err != nil {
		return errors.New("invalid purge-refresh-tokens arguments")
	}
	if *batchSize <= 0 || *timeout <= 0 {
		return errors.New("batch-size and timeout must be positive")
	}
	if *apply && *confirm != purgeConfirmation {
		return errors.New("apply requires the purge confirmation phrase")
	}
	if !*apply && strings.TrimSpace(*confirm) != "" {
		return errors.New("confirm is only accepted together with apply")
	}

	options, err := redisOptionsFromEnvironment()
	if err != nil {
		return err
	}
	client := goredis.NewClient(options)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	result, err := redisinfra.PurgeRefreshTokens(ctx, client, *batchSize, *apply)
	if err != nil {
		return errors.New("refresh token purge failed")
	}
	return writeJSON(output, struct {
		Mode string `json:"mode"`
		redisinfra.RefreshTokenPurgeResult
	}{
		Mode:                    map[bool]string{true: "apply", false: "dry-run"}[*apply],
		RefreshTokenPurgeResult: result,
	})
}

func runDisposeSensitiveLogs(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("dispose-sensitive-logs", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	apply := flags.Bool("apply", false, "apply deletion")
	confirm := flags.String("confirm", "", "required confirmation phrase")
	path := flags.String("path", maintenance.ProductionLogDirectory, "production IAM log directory")
	if err := flags.Parse(args); err != nil {
		return errors.New("invalid dispose-sensitive-logs arguments")
	}
	if *apply && *confirm != logDisposalConfirmation {
		return errors.New("apply requires the log disposal confirmation phrase")
	}
	if !*apply && strings.TrimSpace(*confirm) != "" {
		return errors.New("confirm is only accepted together with apply")
	}
	if err := maintenance.ValidateProductionLogDirectory(*path); err != nil {
		return errors.New("production log directory validation failed")
	}
	plan, err := maintenance.AnalyzeLogDirectory(*path)
	if err != nil {
		return errors.New("sensitive log analysis failed")
	}
	summary := plan.Summary()
	if err := writeJSON(output, struct {
		Mode string `json:"mode"`
		maintenance.LogDisposalSummary
	}{
		Mode:               map[bool]string{true: "apply", false: "dry-run"}[*apply],
		LogDisposalSummary: summary,
	}); err != nil {
		return err
	}
	if !*apply {
		return nil
	}
	applied, err := plan.Dispose()
	if err != nil {
		return errors.New("sensitive log disposal failed")
	}
	return writeJSON(output, struct {
		Mode   string `json:"mode"`
		Result string `json:"result"`
		maintenance.LogDisposalSummary
	}{
		Mode:               "apply",
		Result:             "completed",
		LogDisposalSummary: applied,
	})
}

func redisOptionsFromEnvironment() (*goredis.Options, error) {
	cluster, err := envBool("IAM_APISERVER_REDIS_CACHE_ENABLE_CLUSTER", false)
	if err != nil {
		return nil, err
	}
	if cluster || strings.TrimSpace(os.Getenv("IAM_APISERVER_REDIS_CACHE_ADDRS")) != "" {
		return nil, errors.New("redis cluster mode is not supported")
	}
	host := strings.TrimSpace(envOrDefault("IAM_APISERVER_REDIS_CACHE_HOST", "127.0.0.1"))
	port, err := envInt("IAM_APISERVER_REDIS_CACHE_PORT", 6379)
	if err != nil {
		return nil, err
	}
	database, err := envInt("IAM_APISERVER_REDIS_CACHE_DATABASE", 0)
	if err != nil {
		return nil, err
	}
	useTLS, err := envBool("IAM_APISERVER_REDIS_CACHE_USE_SSL", false)
	if err != nil {
		return nil, err
	}
	insecureSkipVerify, err := envBool("IAM_APISERVER_REDIS_CACHE_SSL_INSECURE_SKIP_VERIFY", false)
	if err != nil {
		return nil, err
	}
	if host == "" || port < 1 || port > 65535 || database < 0 {
		return nil, errors.New("redis cache connection environment is invalid")
	}

	options := &goredis.Options{
		Addr:     net.JoinHostPort(host, strconv.Itoa(port)),
		Username: os.Getenv("IAM_APISERVER_REDIS_CACHE_USERNAME"),
		Password: os.Getenv("IAM_APISERVER_REDIS_CACHE_PASSWORD"),
		DB:       database,
	}
	if useTLS {
		options.TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: insecureSkipVerify, //nolint:gosec // explicit existing deployment option
		}
	}
	return options, nil
}

func envOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) (int, error) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return parsed, nil
}

func envBool(key string, fallback bool) (bool, error) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return parsed, nil
}

func writeJSON(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return errors.New("write maintenance result failed")
	}
	return nil
}

func publicMaintenanceError(err error) string {
	if err == nil {
		return ""
	}
	// Errors returned by this command contain only stable categories or
	// environment key names. Dependency errors are normalized before here.
	return err.Error()
}
