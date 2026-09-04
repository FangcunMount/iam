package jwks

import (
	"context"
	"fmt"
	"testing"
	"time"

	jwks "github.com/FangcunMount/iam/v3/internal/apiserver/infra/token/keyset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB 创建测试数据库
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// 自动迁移
	err = db.AutoMigrate(&KeyPO{})
	require.NoError(t, err)

	return db
}

// createTestKey 创建测试用密钥
func createTestKey(kid string, status jwks.KeyStatus) *jwks.Key {
	notBefore := time.Now().Add(-1 * time.Hour)
	notAfter := time.Now().Add(24 * time.Hour)

	return jwks.NewKey(
		kid,
		jwks.PublicJWK{
			Kty: "RSA",
			Use: "sig",
			Kid: kid,
			Alg: "RS256",
			N:   stringPtr("test-modulus"),
			E:   stringPtr("AQAB"),
		},
		jwks.WithStatus(status),
		jwks.WithNotBefore(notBefore),
		jwks.WithNotAfter(notAfter),
	)
}

func stringPtr(s string) *string {
	return &s
}

func TestKeyRepository_Save(t *testing.T) {
	db := setupTestDB(t)
	repo := NewKeyRepository(db)
	ctx := context.Background()

	key := createTestKey("test-kid-001", jwks.KeyActive)

	err := repo.Save(ctx, key)
	assert.NoError(t, err)

	// 验证保存成功
	found, err := repo.FindByKid(ctx, "test-kid-001")
	require.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, "test-kid-001", found.Kid)
	assert.Equal(t, jwks.KeyActive, found.Status)
}

func TestKeyRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewKeyRepository(db)
	ctx := context.Background()

	// 先保存
	key := createTestKey("test-kid-002", jwks.KeyActive)
	err := repo.Save(ctx, key)
	require.NoError(t, err)

	// 查询并更新状态
	key.Status = jwks.KeyGrace
	err = repo.Update(ctx, key)
	assert.NoError(t, err)

	// 验证更新成功
	found, err := repo.FindByKid(ctx, "test-kid-002")
	require.NoError(t, err)
	assert.Equal(t, jwks.KeyGrace, found.Status)
}

func TestKeyRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewKeyRepository(db)
	ctx := context.Background()

	// 先保存
	key := createTestKey("test-kid-003", jwks.KeyActive)
	err := repo.Save(ctx, key)
	require.NoError(t, err)

	// 删除
	err = repo.Delete(ctx, "test-kid-003")
	assert.NoError(t, err)

	// 验证删除成功
	found, err := repo.FindByKid(ctx, "test-kid-003")
	assert.NoError(t, err)
	assert.Nil(t, found)
}

func TestKeyRepository_FindByKid(t *testing.T) {
	db := setupTestDB(t)
	repo := NewKeyRepository(db)
	ctx := context.Background()

	// 保存测试数据
	key := createTestKey("test-kid-004", jwks.KeyActive)
	err := repo.Save(ctx, key)
	require.NoError(t, err)

	tests := []struct {
		name    string
		kid     string
		wantNil bool
	}{
		{
			name:    "存在的密钥",
			kid:     "test-kid-004",
			wantNil: false,
		},
		{
			name:    "不存在的密钥",
			kid:     "non-existent",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := repo.FindByKid(ctx, tt.kid)
			assert.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, found)
			} else {
				assert.NotNil(t, found)
				assert.Equal(t, tt.kid, found.Kid)
			}
		})
	}
}

func TestKeyRepository_FindByStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := NewKeyRepository(db)
	ctx := context.Background()

	// 保存不同状态的密钥
	keys := []*jwks.Key{
		createTestKey("active-1", jwks.KeyActive),
		createTestKey("active-2", jwks.KeyActive),
		createTestKey("grace-1", jwks.KeyGrace),
		createTestKey("retired-1", jwks.KeyRetired),
	}

	for _, key := range keys {
		err := repo.Save(ctx, key)
		require.NoError(t, err)
	}

	tests := []struct {
		name      string
		status    jwks.KeyStatus
		wantCount int
	}{
		{
			name:      "Active 状态",
			status:    jwks.KeyActive,
			wantCount: 2,
		},
		{
			name:      "Grace 状态",
			status:    jwks.KeyGrace,
			wantCount: 1,
		},
		{
			name:      "Retired 状态",
			status:    jwks.KeyRetired,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := repo.FindByStatus(ctx, tt.status)
			require.NoError(t, err)
			assert.Len(t, found, tt.wantCount)
		})
	}
}

func TestKeyRepository_FindPublishable(t *testing.T) {
	db := setupTestDB(t)
	repo := NewKeyRepository(db)
	ctx := context.Background()

	now := time.Now()

	// 创建不同状态的密钥
	activeKey := createTestKey("publishable-active", jwks.KeyActive)
	graceKey := createTestKey("publishable-grace", jwks.KeyGrace)
	retiredKey := createTestKey("not-publishable-retired", jwks.KeyRetired)

	// 创建已过期的密钥
	expiredKey := createTestKey("expired-key", jwks.KeyActive)
	pastNotBefore := now.Add(-3 * time.Hour)
	pastTime := now.Add(-2 * time.Hour)
	expiredKey.NotBefore = &pastNotBefore
	expiredKey.NotAfter = &pastTime

	keys := []*jwks.Key{activeKey, graceKey, retiredKey, expiredKey}
	for _, key := range keys {
		err := repo.Save(ctx, key)
		require.NoError(t, err)
	}

	// 查询可发布的密钥
	publishable, err := repo.FindPublishable(ctx)
	require.NoError(t, err)

	// 应该只返回 Active 和 Grace 且未过期的密钥
	assert.Len(t, publishable, 2)

	kids := make([]string, len(publishable))
	for i, key := range publishable {
		kids[i] = key.Kid
	}
	assert.Contains(t, kids, "publishable-active")
	assert.Contains(t, kids, "publishable-grace")
}

func TestKeyRepository_FindExpired(t *testing.T) {
	db := setupTestDB(t)
	repo := NewKeyRepository(db)
	ctx := context.Background()

	now := time.Now()

	// 创建已过期的密钥
	expiredKey1 := createTestKey("expired-1", jwks.KeyActive)
	notBefore1 := now.Add(-3 * time.Hour)
	pastTime1 := now.Add(-2 * time.Hour)
	expiredKey1.NotBefore = &notBefore1
	expiredKey1.NotAfter = &pastTime1

	expiredKey2 := createTestKey("expired-2", jwks.KeyGrace)
	notBefore2 := now.Add(-2 * time.Hour)
	pastTime2 := now.Add(-1 * time.Hour)
	expiredKey2.NotBefore = &notBefore2
	expiredKey2.NotAfter = &pastTime2

	// 创建未过期的密钥
	activeKey := createTestKey("active-not-expired", jwks.KeyActive)

	keys := []*jwks.Key{expiredKey1, expiredKey2, activeKey}
	for _, key := range keys {
		err := repo.Save(ctx, key)
		require.NoError(t, err)
	}

	// 查询已过期的密钥
	expired, err := repo.FindExpired(ctx)
	require.NoError(t, err)

	// 应该返回 2 个过期密钥
	assert.Len(t, expired, 2)

	kids := make([]string, len(expired))
	for i, key := range expired {
		kids[i] = key.Kid
	}
	assert.Contains(t, kids, "expired-1")
	assert.Contains(t, kids, "expired-2")
}

func TestKeyRepositoryActivateMovesCurrentActiveToGraceInOneTransaction(t *testing.T) {
	db := setupTestDB(t)
	repo := NewKeyRepository(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	old := createTestKey("old-active", jwks.KeyActive)
	require.NoError(t, repo.Save(ctx, old))
	candidate := createTestKey("new-active", jwks.KeyActive)
	graceUntil := now.Add(7 * 24 * time.Hour)

	result, err := repo.Activate(ctx, jwks.ActivationRequest{
		Candidate:  candidate,
		Now:        now,
		GraceUntil: graceUntil,
	})
	require.NoError(t, err)
	require.True(t, result.Activated)
	require.Equal(t, "new-active", result.Active.Kid)

	oldAfter, err := repo.FindByKid(ctx, "old-active")
	require.NoError(t, err)
	require.Equal(t, jwks.KeyGrace, oldAfter.Status)
	require.WithinDuration(t, graceUntil, *oldAfter.NotAfter, time.Second)
	active, err := repo.FindByStatus(ctx, jwks.KeyActive)
	require.NoError(t, err)
	require.Len(t, active, 1)
	require.Equal(t, "new-active", active[0].Kid)
}

func TestKeyRepositoryActivateIfDueNoopsAfterConcurrentWinner(t *testing.T) {
	db := setupTestDB(t)
	repo := NewKeyRepository(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	active := createTestKey("fresh-active", jwks.KeyActive)
	notBefore := now.Add(-time.Hour)
	active.NotBefore = &notBefore
	require.NoError(t, repo.Save(ctx, active))
	candidate := createTestKey("unused-candidate", jwks.KeyActive)
	dueBefore := now.Add(-30 * 24 * time.Hour)

	result, err := repo.Activate(ctx, jwks.ActivationRequest{
		Candidate:  candidate,
		Now:        now,
		GraceUntil: now.Add(7 * 24 * time.Hour),
		DueBefore:  &dueBefore,
	})
	require.NoError(t, err)
	require.False(t, result.Activated)
	require.Equal(t, "fresh-active", result.Active.Kid)
	found, err := repo.FindByKid(ctx, "unused-candidate")
	require.NoError(t, err)
	require.Nil(t, found)
}

func TestKeyRepository_FindAll(t *testing.T) {
	db := setupTestDB(t)
	repo := NewKeyRepository(db)
	ctx := context.Background()

	// 保存 5 个密钥
	for i := 1; i <= 5; i++ {
		key := createTestKey(fmt.Sprintf("key-%d", i), jwks.KeyActive)
		err := repo.Save(ctx, key)
		require.NoError(t, err)
	}

	tests := []struct {
		name      string
		offset    int
		limit     int
		wantCount int
		wantTotal int64
	}{
		{
			name:      "查询全部",
			offset:    0,
			limit:     10,
			wantCount: 5,
			wantTotal: 5,
		},
		{
			name:      "第一页",
			offset:    0,
			limit:     3,
			wantCount: 3,
			wantTotal: 5,
		},
		{
			name:      "第二页",
			offset:    3,
			limit:     3,
			wantCount: 2,
			wantTotal: 5,
		},
		{
			name:      "仅 offset 也能工作",
			offset:    2,
			limit:     0,
			wantCount: 3,
			wantTotal: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys, total, err := repo.FindAll(ctx, tt.limit, tt.offset)
			require.NoError(t, err)
			assert.Len(t, keys, tt.wantCount)
			assert.Equal(t, tt.wantTotal, total)
		})
	}
}

func TestKeyRepository_CountByStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := NewKeyRepository(db)
	ctx := context.Background()

	// 保存不同状态的密钥
	keys := []*jwks.Key{
		createTestKey("count-active-1", jwks.KeyActive),
		createTestKey("count-active-2", jwks.KeyActive),
		createTestKey("count-active-3", jwks.KeyActive),
		createTestKey("count-grace-1", jwks.KeyGrace),
		createTestKey("count-retired-1", jwks.KeyRetired),
	}

	for _, key := range keys {
		err := repo.Save(ctx, key)
		require.NoError(t, err)
	}

	tests := []struct {
		name      string
		status    jwks.KeyStatus
		wantCount int64
	}{
		{
			name:      "Active 数量",
			status:    jwks.KeyActive,
			wantCount: 3,
		},
		{
			name:      "Grace 数量",
			status:    jwks.KeyGrace,
			wantCount: 1,
		},
		{
			name:      "Retired 数量",
			status:    jwks.KeyRetired,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := repo.CountByStatus(ctx, tt.status)
			require.NoError(t, err)
			assert.Equal(t, tt.wantCount, count)
		})
	}
}
