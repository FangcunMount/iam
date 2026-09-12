package scopemigrate

import (
	"database/sql"
	"fmt"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"strings"
	"testing"
	"time"
)

// Every test creates isolated databases. A DSN naming any existing database is
// rejected before connecting; no production schema may be reused for this suite.
func mysqlTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	raw := os.Getenv("SCOPE_MIGRATION_MYSQL_DSN")
	if raw == "" {
		if os.Getenv("SCOPE_MIGRATION_REQUIRE_MYSQL") == "true" {
			t.Fatal("SCOPE_MIGRATION_MYSQL_DSN required")
		}
		return nil
	}
	cfg, err := driver.ParseDSN(raw)
	if err != nil {
		t.Fatal("invalid isolated MySQL DSN")
	}
	if cfg.DBName != "" {
		t.Fatal("test DSN must not select an existing database")
	}
	cfg.ParseTime = true
	cfg.MultiStatements = true
	cfg.Loc = time.UTC
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("cannot open isolated MySQL admin")
	}
	name := fmt.Sprintf("iam_scope_test_%d", time.Now().UnixNano())
	if !strings.HasPrefix(name, "iam_scope_test_") {
		t.Fatal("unsafe test database")
	}
	if _, err := admin.Exec("CREATE DATABASE `" + name + "`"); err != nil {
		if closeErr := admin.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal(err)
	}
	cfg.DBName = name
	db, err := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		if _, e := admin.Exec("DROP DATABASE `" + name + "`"); e != nil {
			t.Error(e)
		}
		if closeErr := admin.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal("cannot connect to isolated MySQL test database")
	}
	t.Cleanup(func() {
		pool, _ := db.DB()
		if closeErr := pool.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		if _, e := admin.Exec("DROP DATABASE `" + name + "`"); e != nil {
			t.Error(e)
		}
		if closeErr := admin.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	return db
}
