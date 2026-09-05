// Package authzdb creates disposable databases for authorization contract tests.
package authzdb

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	assignmentrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/assignment"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/eventoutbox"
	grantrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/permissiongrant"
	policyrepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/policy"
	resourcerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/resource"
	rolerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	inheritancerepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/roleinheritance"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(t *testing.T, mysqlRequired bool) *gorm.DB {
	t.Helper()
	config := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	var db *gorm.DB
	var err error
	if mysqlRequired {
		dsn := os.Getenv("IAM_AUTHZ_TEST_MYSQL_DSN")
		if dsn == "" {
			t.Skip("IAM_AUTHZ_TEST_MYSQL_DSN is not configured")
		}
		c, err := mysql.ParseDSN(dsn)
		require.NoError(t, err)
		c.DBName = ""
		c.ParseTime = true
		admin, err := gorm.Open(gormmysql.Open(c.FormatDSN()), config)
		require.NoError(t, err)
		name := fmt.Sprintf("iam_authz_hardening_test_%d", time.Now().UnixNano())
		require.NoError(t, admin.Exec("CREATE DATABASE "+name).Error)
		t.Cleanup(func() {
			require.NoError(t, admin.Exec("DROP DATABASE "+name).Error)
			sqlDB, _ := admin.DB()
			_ = sqlDB.Close()
		})
		c.DBName = name
		db, err = gorm.Open(gormmysql.Open(c.FormatDSN()), config)
		require.NoError(t, err)
	} else {
		db, err = gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "authz.db")), config)
		require.NoError(t, err)
	}
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if !mysqlRequired {
		sqlDB.SetMaxOpenConns(1)
	}
	require.NoError(t, db.AutoMigrate(&rolerepo.RolePO{}, &resourcerepo.ResourcePO{}, &assignmentrepo.AssignmentPO{}, &inheritancerepo.InheritancePO{}, &grantrepo.GrantPO{}, &policyrepo.PolicyVersionPO{}, &eventoutbox.OutboxPO{}))
	require.NoError(t, db.Exec("CREATE TABLE users (id BIGINT PRIMARY KEY, status INT NOT NULL, deleted_at DATETIME NULL)").Error)
	return db
}
