package app

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestPrintViperConfigDoesNotPrintConfigurationOrEnvironmentValues(t *testing.T) {
	const (
		configSecret = "config-secret-sentinel"
		envValue     = "environment-value-sentinel"
	)

	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("mysql.password", configSecret)
	t.Setenv("IAM_APISERVER_MYSQL_HOST", envValue)

	output := captureStdout(t, func() {
		printViperConfig("iam-apiserver")
	})

	for _, secret := range []string{configSecret, envValue} {
		if strings.Contains(output, secret) {
			t.Fatalf("configuration diagnostics leaked %q:\n%s", secret, output)
		}
	}
	for _, want := range []string{
		"mysql.password",
		"ENV IAM_APISERVER_MYSQL_HOST=<set>",
		"ENV IAM_APISERVER_MYSQL_DATABASE=<unset>",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("configuration diagnostics missing %q:\n%s", want, output)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = original
		_ = reader.Close()
	}()

	fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	return string(output)
}
