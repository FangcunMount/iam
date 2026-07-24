package process

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	componentlogger "github.com/FangcunMount/component-base/pkg/logger"
)

func TestProductionGORMLoggerDoesNotEvaluateOrLogSQL(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "gorm-security.log")
	options := log.NewOptions()
	options.Format = "json"
	options.OutputPaths = []string{logPath}
	options.ErrorOutputPaths = []string{logPath}
	log.Init(options)
	t.Cleanup(func() {
		log.Flush()
		log.Init(log.NewOptions())
	})

	const sqlSentinel = "SELECT * FROM users WHERE phone = 'sql-phone-sentinel'"
	evaluated := false
	gormLogger := componentlogger.NewGormLogger(1)
	gormLogger.Trace(context.Background(), time.Now().Add(-time.Second), func() (string, int64) {
		evaluated = true
		return sqlSentinel, 0
	}, errors.New("gorm-error-sentinel"))
	log.Flush()

	if evaluated {
		t.Fatal("silent GORM logger evaluated SQL callback")
	}
	content, err := os.ReadFile(logPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, forbidden := range []string{sqlSentinel, "gorm-error-sentinel", "GORM trace"} {
		if strings.Contains(string(content), forbidden) {
			t.Fatalf("silent GORM logger leaked %q: %s", forbidden, content)
		}
	}
}

func TestDevelopmentGORMInfoStillEmitsSQL(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "gorm-development.log")
	options := log.NewOptions()
	options.Level = "debug"
	options.Format = "json"
	options.OutputPaths = []string{logPath}
	options.ErrorOutputPaths = []string{logPath}
	log.Init(options)
	t.Cleanup(func() {
		log.Flush()
		log.Init(log.NewOptions())
	})

	const sqlSentinel = "SELECT development-gorm-sentinel"
	evaluated := false
	gormLogger := componentlogger.NewGormLogger(4)
	gormLogger.Trace(context.Background(), time.Now(), func() (string, int64) {
		evaluated = true
		return sqlSentinel, 1
	}, nil)
	log.Flush()

	if !evaluated {
		t.Fatal("development GORM logger did not evaluate SQL callback")
	}
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), sqlSentinel) {
		t.Fatalf("development GORM logger did not emit SQL: %s", content)
	}
}
