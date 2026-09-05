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

	redisinfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/cache/redis"
	eventoutbox "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/eventoutbox"
	authzmysqluow "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v3/internal/apiserver/maintenance"
	"github.com/FangcunMount/iam/v3/pkg/eventcatalog"
	goredis "github.com/redis/go-redis/v9"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	purgeConfirmation       = "PURGE_REFRESH_TOKENS"
	logDisposalConfirmation = "DELETE_PRE_5_4_IAM_LOGS"
	authzV3ConvergeCatalog  = "configs/events.yaml"
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
	case "authz-hardening":
		return runAuthzHardening(args[1:], output)
	case "authz-v3-converge":
		return runAuthzV3Converge(args[1:], output)
	default:
		return errors.New("unsupported maintenance subcommand")
	}
}

func runAuthzV3Converge(args []string, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("an authz-v3-converge operation is required")
	}
	operation := strings.TrimSpace(args[0])
	if operation != "preflight" && operation != "apply" && operation != "verify" && operation != "evidence" {
		return errors.New("unsupported authz-v3-converge operation")
	}
	flags := flag.NewFlagSet("authz-v3-converge "+operation, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	confirm := flags.String("confirm", "", "required apply confirmation phrase")
	expectedSourceHash := flags.String("expected-source-hash", "", "approved preflight state hash")
	timeout := flags.Duration("timeout", 10*time.Minute, "operation timeout")
	lockTimeout := flags.Duration("lock-timeout", 30*time.Second, "database named-lock timeout")
	eventsCatalog := flags.String("events-catalog", envOrDefault("IAM_EVENTS_CATALOG_PATH", authzV3ConvergeCatalog), "event catalog path")
	buildSHA := flags.String("build-sha", strings.TrimSpace(firstEnvironment("IAM_BUILD_SHA", "GITHUB_SHA")), "exact candidate build SHA")
	if err := flags.Parse(args[1:]); err != nil {
		return errors.New("invalid authz-v3-converge arguments")
	}
	if *timeout <= 0 || *lockTimeout <= 0 {
		return errors.New("timeout and lock-timeout must be positive")
	}
	if operation == "evidence" && strings.TrimSpace(*buildSHA) == "" {
		return errors.New("evidence requires an exact build sha")
	}
	if operation == "apply" {
		if *confirm != maintenance.AuthzV3ConvergeConfirmation {
			return errors.New("apply requires the authz v3 convergence confirmation phrase")
		}
		if strings.TrimSpace(*expectedSourceHash) == "" {
			return errors.New("apply requires the approved source hash")
		}
	} else if strings.TrimSpace(*confirm) != "" || strings.TrimSpace(*expectedSourceHash) != "" {
		return errors.New("confirm and expected-source-hash are accepted only for apply")
	}

	db, err := authzConvergeDatabaseFromEnvironment()
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return errors.New("authorization database initialization failed")
	}
	defer func() { _ = sqlDB.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if operation == "apply" {
		release, lockErr := acquireAuthzV3ConvergeLock(ctx, sqlDB, *lockTimeout)
		if lockErr != nil {
			return lockErr
		}
		defer release()
	}

	var plan *maintenance.AuthzV3ConvergePlan
	switch operation {
	case "preflight":
		plan, err = maintenance.AnalyzeAuthzV3Convergence(ctx, db)
	case "apply":
		catalogConfig, loadErr := eventcatalog.Load(strings.TrimSpace(*eventsCatalog))
		if loadErr != nil {
			return errors.New("authorization event catalog load failed")
		}
		catalog := eventcatalog.NewCatalog(catalogConfig)
		stager := eventoutbox.NewStore(db, catalog)
		uow := authzmysqluow.NewUnitOfWork(db, nil, stager)
		plan, err = maintenance.ApplyAuthzV3Convergence(ctx, db, uow, *expectedSourceHash)
	case "verify", "evidence":
		plan, err = maintenance.VerifyAuthzV3Convergence(ctx, db)
	}
	if plan != nil {
		writeErr := writeJSON(output, struct {
			Operation string `json:"operation"`
			BuildSHA  string `json:"build_sha,omitempty"`
			maintenance.AuthzV3ConvergeSummary
		}{Operation: operation, BuildSHA: strings.TrimSpace(*buildSHA), AuthzV3ConvergeSummary: plan.Summary})
		if writeErr != nil {
			return writeErr
		}
	}
	if err != nil {
		return errors.New("authorization v3 convergence " + operation + " failed")
	}
	if plan == nil {
		return errors.New("authorization v3 convergence produced no result")
	}
	if len(plan.Summary.Blockers) > 0 {
		return errors.New("authorization v3 convergence is blocked")
	}
	return nil
}

func authzConvergeDatabaseFromEnvironment() (*gorm.DB, error) {
	host := strings.TrimSpace(envOrDefault("MYSQL_HOST", "127.0.0.1"))
	port, err := envInt("MYSQL_PORT", 3306)
	if err != nil {
		return nil, err
	}
	username := strings.TrimSpace(firstEnvironment("MYSQL_USER", "MYSQL_USERNAME"))
	password := firstEnvironment("MYSQL_PASSWORD")
	database := strings.TrimSpace(firstEnvironment("MYSQL_DATABASE", "MYSQL_DBNAME"))
	if host == "" || port < 1 || port > 65535 || username == "" || database == "" {
		return nil, errors.New("authorization database connection environment is invalid")
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true",
		username, password, net.JoinHostPort(host, strconv.Itoa(port)), database,
	)
	db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, errors.New("authorization database connection failed")
	}
	return db, nil
}

func acquireAuthzV3ConvergeLock(ctx context.Context, db *sql.DB, timeout time.Duration) (func(), error) {
	connection, err := db.Conn(ctx)
	if err != nil {
		return nil, errors.New("authorization convergence database lock unavailable")
	}
	waitSeconds := int((timeout + time.Second - 1) / time.Second)
	var acquired int
	if err := connection.QueryRowContext(ctx, "SELECT GET_LOCK(?, ?)", maintenance.AuthzV3ConvergeLockName, waitSeconds).Scan(&acquired); err != nil || acquired != 1 {
		_ = connection.Close()
		return nil, errors.New("authorization convergence database lock unavailable")
	}
	return func() {
		var released int
		_ = connection.QueryRowContext(context.Background(), "SELECT RELEASE_LOCK(?)", maintenance.AuthzV3ConvergeLockName).Scan(&released)
		_ = connection.Close()
	}, nil
}

func firstEnvironment(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); strings.TrimSpace(value) != "" {
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
