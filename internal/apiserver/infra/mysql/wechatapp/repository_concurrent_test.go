package mysql

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/apiserver/domain/idp/wechatapp"
	testhelpers "github.com/FangcunMount/iam/internal/apiserver/testhelpers"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/stretchr/testify/require"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// 并发创建相同的 wechat app（相同 app_id），期望只有 1 条记录被写入，
// 其余并发请求因唯一约束被 translator 映射为业务错误 code.ErrWechatAppAlreadyExists。
func TestWechatAppRepository_Create_ConcurrentDuplicateDetection(t *testing.T) {
	var db *gorm.DB
	var err error

	// 如果设置了 MYSQL_HOST 则使用 MySQL（适用于本地 Docker 容器）；否则使用 sqlite in-memory
	if os.Getenv("MYSQL_HOST") != "" {
		host := os.Getenv("MYSQL_HOST")
		port := os.Getenv("MYSQL_PORT")
		if port == "" {
			port = "3306"
		}
		user := os.Getenv("MYSQL_USER")
		pass := os.Getenv("MYSQL_PASSWORD")
		dbName := os.Getenv("MYSQL_DATABASE")
		if dbName == "" {
			dbName = os.Getenv("MYSQL_DBNAME")
		}
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&loc=Local", user, pass, host, port, dbName)
		db, err = gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
		require.NoError(t, err)

		// 确保表结构存在并清理测试数据
		require.NoError(t, db.AutoMigrate(&WechatAppPO{}))
		require.NoError(t, db.Exec("DELETE FROM idp_wechat_apps WHERE app_id = ?", "app-dup").Error)
	} else {
		db = testhelpers.SetupTempSQLiteDB(t)
		require.NoError(t, db.AutoMigrate(&WechatAppPO{}))
	}

	repo := NewWechatAppRepository(db)
	ctx := context.Background()

	const concurrency = 100
	var wg sync.WaitGroup
	wg.Add(concurrency)

	errs := make(chan error, concurrency)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < concurrency; i++ {
		// compute delay in the parent goroutine to avoid concurrent access to rng
		delay := rng.Intn(8)
		go func(d int) {
			defer wg.Done()
			time.Sleep(time.Millisecond * time.Duration(d))
			app := wechatapp.NewWechatApp(wechatapp.AppType("minip"), "app-dup", wechatapp.WithWechatAppName("concurrent"))
			if err := testhelpers.RetryOnDBLocked(func() error { return repo.Create(ctx, app) }); err != nil {
				errs <- err
				return
			}
			errs <- nil
		}(delay)
	}

	wg.Wait()
	close(errs)

	var success int
	var mappedCount int
	for e := range errs {
		if e != nil {
			t.Logf("err: %T: %v", e, e)
		}
		if e == nil {
			success++
			continue
		}

		var ue error = e
		for ue != nil {
			if perrors.IsCode(ue, code.ErrWechatAppAlreadyExists) {
				mappedCount++
				break
			}
			ue = errors.Unwrap(ue)
		}
	}

	require.Equal(t, 1, success, "only one create should succeed")
	require.GreaterOrEqual(t, mappedCount, 1, "at least one error should be mapped to ErrWechatAppAlreadyExists")

	var cnt int64
	require.NoError(t, db.Model(&WechatAppPO{}).
		Where("app_id = ?", "app-dup").
		Count(&cnt).Error)
	require.Equal(t, int64(1), cnt)
}
