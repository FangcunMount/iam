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

	redisinfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/cache/redis"
	"github.com/FangcunMount/iam/v2/internal/apiserver/maintenance"
	goredis "github.com/redis/go-redis/v9"
)

const (
	purgeConfirmation       = "PURGE_REFRESH_TOKENS"
	logDisposalConfirmation = "DELETE_PRE_5_4_IAM_LOGS"
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
	default:
		return errors.New("unsupported maintenance subcommand")
	}
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
