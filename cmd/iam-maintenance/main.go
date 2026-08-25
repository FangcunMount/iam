package main

import (
	"context"
	"crypto/tls"
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
	"github.com/FangcunMount/iam/v3/internal/apiserver/maintenance"
	goredis "github.com/redis/go-redis/v9"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	purgeConfirmation        = "PURGE_REFRESH_TOKENS"
	logDisposalConfirmation  = "DELETE_PRE_5_4_IAM_LOGS"
	authzCutoverConfirmation = "APPLY_AUTHZ_CUTOVER"
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
	case "authz-cutover":
		return runAuthzCutover(args[1:], output)
	default:
		return errors.New("unsupported maintenance subcommand")
	}
}

func runAuthzCutover(args []string, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("an authz-cutover operation is required")
	}
	operation := args[0]
	flags := flag.NewFlagSet("authz-cutover "+operation, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	confirm := flags.String("confirm", "", "required apply confirmation phrase")
	timeout := flags.Duration("timeout", 10*time.Minute, "operation timeout")
	lockTimeout := flags.Duration("lock-timeout", 30*time.Second, "database named-lock timeout")
	if err := flags.Parse(args[1:]); err != nil {
		return errors.New("invalid authz-cutover arguments")
	}
	if *timeout <= 0 || *lockTimeout <= 0 {
		return errors.New("timeout and lock-timeout must be positive")
	}
	if operation == "apply" {
		if *confirm != authzCutoverConfirmation {
			return errors.New("apply requires the authz cutover confirmation phrase")
		}
	} else if strings.TrimSpace(*confirm) != "" {
		return errors.New("confirm is only accepted for apply")
	}
	if operation != "preflight" && operation != "apply" && operation != "verify" && operation != "evidence" {
		return errors.New("unsupported authz-cutover operation")
	}

	db, err := authzCutoverDatabaseFromEnvironment()
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
		release, lockErr := acquireAuthzCutoverLock(ctx, db, *lockTimeout)
		if lockErr != nil {
			return lockErr
		}
		defer release()
	}
	plan, err := maintenance.AnalyzeAuthzCutover(ctx, db)
	if err != nil {
		return errors.New("authorization cutover analysis failed")
	}
	if len(plan.Summary.Blockers) > 0 {
		if writeErr := writeJSON(output, struct {
			Operation string `json:"operation"`
			maintenance.AuthzCutoverSummary
		}{Operation: operation, AuthzCutoverSummary: plan.Summary}); writeErr != nil {
			return writeErr
		}
		return errors.New("authorization cutover is blocked")
	}

	var summary maintenance.AuthzCutoverSummary
	switch operation {
	case "preflight":
		summary = plan.Summary
	case "apply":
		summary, err = maintenance.ApplyAuthzCutover(ctx, db, plan)
	case "verify", "evidence":
		summary, err = maintenance.VerifyAuthzCutover(ctx, db, plan)
	}
	if err != nil {
		return errors.New("authorization cutover " + operation + " failed")
	}
	return writeJSON(output, struct {
		Operation string `json:"operation"`
		maintenance.AuthzCutoverSummary
	}{Operation: operation, AuthzCutoverSummary: summary})
}

func authzCutoverDatabaseFromEnvironment() (*gorm.DB, error) {
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

func acquireAuthzCutoverLock(ctx context.Context, db *gorm.DB, timeout time.Duration) (func(), error) {
	var acquired int
	if err := db.WithContext(ctx).Raw("SELECT GET_LOCK(?, ?)", maintenance.AuthzCutoverLockName, int(timeout.Seconds())).Scan(&acquired).Error; err != nil || acquired != 1 {
		return nil, errors.New("authorization cutover database lock unavailable")
	}
	return func() {
		var released int
		_ = db.Raw("SELECT RELEASE_LOCK(?)", maintenance.AuthzCutoverLockName).Scan(&released).Error
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
