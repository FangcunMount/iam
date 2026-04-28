package guardianship_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/internal/apiserver/application/uc/child"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/guardianship"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/testutil"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/user"
)

// ==================== GuardianshipApplicationService 测试 ====================

func TestGuardianshipApplicationService_AddGuardian_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := uow.NewUnitOfWork(db)
	ctx := context.Background()

	// 先创建用户和儿童
	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "张三",
		Phone: "13800138000",
		Email: "zhang3@test.com",
	})
	require.NoError(t, err)

	childService := child.NewChildApplicationService(unitOfWork)
	childResult, err := childService.Register(ctx, child.RegisterChildDTO{
		Name:     "小明",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	guardianshipService := guardianship.NewGuardianshipApplicationService(unitOfWork)

	// Act - 添加监护关系
	dto := guardianship.AddGuardianDTO{
		UserID:   userResult.ID,
		ChildID:  childResult.ID,
		Relation: "parent",
	}
	err = guardianshipService.AddGuardian(ctx, dto)

	// Assert
	require.NoError(t, err)

	// 验证监护关系是否创建成功
	queryService := guardianship.NewGuardianshipQueryApplicationService(unitOfWork)
	isGuardian, err := queryService.IsGuardian(ctx, userResult.ID, childResult.ID)
	require.NoError(t, err)
	assert.True(t, isGuardian)
}

func TestGuardianshipApplicationService_AddGuardian_DuplicateGuardian(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := uow.NewUnitOfWork(db)
	ctx := context.Background()

	// 先创建用户和儿童
	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "李四",
		Phone: "13800138001",
		Email: "li4@test.com",
	})
	require.NoError(t, err)

	childService := child.NewChildApplicationService(unitOfWork)
	childResult, err := childService.Register(ctx, child.RegisterChildDTO{
		Name:     "小红",
		Gender:   2,
		Birthday: "2019-05-20",
	})
	require.NoError(t, err)

	guardianshipService := guardianship.NewGuardianshipApplicationService(unitOfWork)

	// 第一次添加监护关系
	dto := guardianship.AddGuardianDTO{
		UserID:   userResult.ID,
		ChildID:  childResult.ID,
		Relation: "parent",
	}
	err = guardianshipService.AddGuardian(ctx, dto)
	require.NoError(t, err)

	// Act - 尝试重复添加相同的监护关系
	err = guardianshipService.AddGuardian(ctx, dto)

	// Assert - 应该失败
	require.Error(t, err)
}

func TestGuardianshipApplicationService_RemoveGuardian_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := uow.NewUnitOfWork(db)
	ctx := context.Background()

	// 先创建用户、儿童和监护关系
	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "王五",
		Phone: "13800138002",
		Email: "wang5@test.com",
	})
	require.NoError(t, err)

	childService := child.NewChildApplicationService(unitOfWork)
	childResult, err := childService.Register(ctx, child.RegisterChildDTO{
		Name:     "小强",
		Gender:   1,
		Birthday: "2021-03-10",
	})
	require.NoError(t, err)

	guardianshipService := guardianship.NewGuardianshipApplicationService(unitOfWork)
	err = guardianshipService.AddGuardian(ctx, guardianship.AddGuardianDTO{
		UserID:   userResult.ID,
		ChildID:  childResult.ID,
		Relation: "parent",
	})
	require.NoError(t, err)

	// Act - 移除监护关系
	dto := guardianship.RemoveGuardianDTO{
		UserID:  userResult.ID,
		ChildID: childResult.ID,
	}
	err = guardianshipService.RemoveGuardian(ctx, dto)

	// Assert
	require.NoError(t, err)

	// 验证监护关系是否已移除
	queryService := guardianship.NewGuardianshipQueryApplicationService(unitOfWork)
	result, err := queryService.GetByUserIDAndChildID(ctx, userResult.ID, childResult.ID)
	require.Error(t, err)
	assert.Nil(t, result)

	historical, err := queryService.GetByUserIDAndChildIDIncludingRevoked(ctx, userResult.ID, childResult.ID)
	require.NoError(t, err)
	require.NotNil(t, historical)
	assert.NotEmpty(t, historical.RevokedAt)

	isGuardian, err := queryService.IsGuardian(ctx, userResult.ID, childResult.ID)
	require.NoError(t, err)
	assert.False(t, isGuardian)
}

func TestGuardianshipApplicationService_RemoveGuardian_NotFound(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := uow.NewUnitOfWork(db)
	guardianshipService := guardianship.NewGuardianshipApplicationService(unitOfWork)
	ctx := context.Background()

	// Act - 尝试移除不存在的监护关系
	dto := guardianship.RemoveGuardianDTO{
		UserID:  "999999999999999999",
		ChildID: "888888888888888888",
	}
	err := guardianshipService.RemoveGuardian(ctx, dto)

	// Assert - 应该失败
	require.Error(t, err)
}

func TestGuardianshipQueryApplicationService_IsGuardian_True(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := uow.NewUnitOfWork(db)
	ctx := context.Background()

	// 创建用户、儿童和监护关系
	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "孙七",
		Phone: "13800138004",
		Email: "sun7@test.com",
	})
	require.NoError(t, err)

	childService := child.NewChildApplicationService(unitOfWork)
	childResult, err := childService.Register(ctx, child.RegisterChildDTO{
		Name:     "小虎",
		Gender:   1,
		Birthday: "2020-08-20",
	})
	require.NoError(t, err)

	guardianshipService := guardianship.NewGuardianshipApplicationService(unitOfWork)
	err = guardianshipService.AddGuardian(ctx, guardianship.AddGuardianDTO{
		UserID:   userResult.ID,
		ChildID:  childResult.ID,
		Relation: "parent",
	})
	require.NoError(t, err)

	queryService := guardianship.NewGuardianshipQueryApplicationService(unitOfWork)

	// Act - 检查是否为监护人
	isGuardian, err := queryService.IsGuardian(ctx, userResult.ID, childResult.ID)

	// Assert
	require.NoError(t, err)
	assert.True(t, isGuardian)
}

func TestGuardianshipQueryApplicationService_IsGuardian_False(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := uow.NewUnitOfWork(db)
	queryService := guardianship.NewGuardianshipQueryApplicationService(unitOfWork)
	ctx := context.Background()

	// Act - 检查不存在的监护关系
	isGuardian, err := queryService.IsGuardian(ctx, "999999999999999999", "888888888888888888")

	// Assert
	require.NoError(t, err)
	assert.False(t, isGuardian)
}

func TestGuardianshipQueryApplicationService_GetByUserIDAndChildID_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := uow.NewUnitOfWork(db)
	ctx := context.Background()

	// 创建用户、儿童和监护关系
	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "周八",
		Phone: "13800138005",
		Email: "zhou8@test.com",
	})
	require.NoError(t, err)

	childService := child.NewChildApplicationService(unitOfWork)
	childResult, err := childService.Register(ctx, child.RegisterChildDTO{
		Name:     "小龙",
		Gender:   1,
		Birthday: "2019-12-25",
	})
	require.NoError(t, err)

	guardianshipService := guardianship.NewGuardianshipApplicationService(unitOfWork)
	err = guardianshipService.AddGuardian(ctx, guardianship.AddGuardianDTO{
		UserID:   userResult.ID,
		ChildID:  childResult.ID,
		Relation: "grandparent",
	})
	require.NoError(t, err)

	queryService := guardianship.NewGuardianshipQueryApplicationService(unitOfWork)

	// Act - 查询监护关系
	result, err := queryService.GetByUserIDAndChildID(ctx, userResult.ID, childResult.ID)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, userResult.ID, result.UserID)
	assert.Equal(t, childResult.ID, result.ChildID)
	assert.Equal(t, "小龙", result.ChildName)
}

func TestGuardianshipQueryApplicationService_GetByUserIDAndChildID_NotFound(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := uow.NewUnitOfWork(db)
	queryService := guardianship.NewGuardianshipQueryApplicationService(unitOfWork)
	ctx := context.Background()

	// Act - 查询不存在的监护关系
	result, err := queryService.GetByUserIDAndChildID(ctx, "999999999999999999", "888888888888888888")

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestGuardianshipQueryApplicationService_ListChildrenByUserID_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := uow.NewUnitOfWork(db)
	ctx := context.Background()

	// 创建用户
	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "吴九",
		Phone: "13800138006",
		Email: "wu9@test.com",
	})
	require.NoError(t, err)

	// 创建多个儿童
	childService := child.NewChildApplicationService(unitOfWork)
	child1, err := childService.Register(ctx, child.RegisterChildDTO{
		Name:     "大宝",
		Gender:   1,
		Birthday: "2018-01-01",
		IDCard:   "110101201801011112",
	})
	require.NoError(t, err)

	child2, err := childService.Register(ctx, child.RegisterChildDTO{
		Name:     "二宝",
		Gender:   2,
		Birthday: "2020-06-01",
		IDCard:   "110101202006012225",
	})
	require.NoError(t, err)

	// 添加监护关系
	guardianshipService := guardianship.NewGuardianshipApplicationService(unitOfWork)
	err = guardianshipService.AddGuardian(ctx, guardianship.AddGuardianDTO{
		UserID:   userResult.ID,
		ChildID:  child1.ID,
		Relation: "parent",
	})
	require.NoError(t, err)

	err = guardianshipService.AddGuardian(ctx, guardianship.AddGuardianDTO{
		UserID:   userResult.ID,
		ChildID:  child2.ID,
		Relation: "parent",
	})
	require.NoError(t, err)

	queryService := guardianship.NewGuardianshipQueryApplicationService(unitOfWork)

	// Act - 列出用户的所有儿童
	results, err := queryService.ListChildrenByUserID(ctx, userResult.ID)

	// Assert
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestGuardianshipQueryApplicationService_ListGuardiansByChildID_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := uow.NewUnitOfWork(db)
	ctx := context.Background()

	// 创建多个用户 (设置唯一的email避免UNIQUE约束冲突)
	userService := user.NewUserApplicationService(unitOfWork)
	user1, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "爸爸",
		Phone: "13800138007",
		Email: "father@example.com", // 唯一email
	})
	require.NoError(t, err)

	// 设置唯一的 IDCard 避免 UNIQUE 约束冲突
	profileService := user.NewUserProfileApplicationService(unitOfWork)
	err = profileService.UpdateIDCard(ctx, user1.ID, "320106198001011110")
	require.NoError(t, err)

	user2, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "妈妈",
		Phone: "13800138008",
		Email: "mother@example.com", // 唯一email
	})
	require.NoError(t, err)

	// 设置唯一的 IDCard 避免 UNIQUE 约束冲突
	err = profileService.UpdateIDCard(ctx, user2.ID, "320106198001012228")
	require.NoError(t, err)

	// 创建儿童
	childService := child.NewChildApplicationService(unitOfWork)
	childResult, err := childService.Register(ctx, child.RegisterChildDTO{
		Name:     "宝宝",
		Gender:   2,
		Birthday: "2021-01-01",
	})
	require.NoError(t, err)

	// 添加多个监护人
	guardianshipService := guardianship.NewGuardianshipApplicationService(unitOfWork)
	err = guardianshipService.AddGuardian(ctx, guardianship.AddGuardianDTO{
		UserID:   user1.ID,
		ChildID:  childResult.ID,
		Relation: "parent",
	})
	require.NoError(t, err)

	err = guardianshipService.AddGuardian(ctx, guardianship.AddGuardianDTO{
		UserID:   user2.ID,
		ChildID:  childResult.ID,
		Relation: "parent",
	})
	require.NoError(t, err)

	queryService := guardianship.NewGuardianshipQueryApplicationService(unitOfWork)

	// Act - 列出儿童的所有监护人
	results, err := queryService.ListGuardiansByChildID(ctx, childResult.ID)

	// Assert
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestGuardianshipApplicationService_AddGuardian_ConcurrentPersistence_10(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := uow.NewUnitOfWork(db)
	ctx := context.Background()

	// 创建用户和儿童（使用唯一 email 避免 UNIQUE 冲突）
	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "并发父亲",
		Phone: "13900000000",
		Email: "concurrent_father@example.com",
	})
	require.NoError(t, err)

	childService := child.NewChildApplicationService(unitOfWork)
	childResult, err := childService.Register(ctx, child.RegisterChildDTO{
		Name:     "并发孩子",
		Gender:   1,
		Birthday: "2020-02-02",
	})
	require.NoError(t, err)

	guardianshipService := guardianship.NewGuardianshipApplicationService(unitOfWork)
	queryService := guardianship.NewGuardianshipQueryApplicationService(unitOfWork)

	// Act - 并发发起 10 个添加监护请求
	const N = 10
	var wg sync.WaitGroup
	wg.Add(N)

	// 为了提高并发命中率，让 goroutine 等待同一个开始信号
	start := make(chan struct{})

	for i := 0; i < N; i++ {
		go func(idx int) {
			defer wg.Done()
			<-start
			dto := guardianship.AddGuardianDTO{
				UserID:   userResult.ID,
				ChildID:  childResult.ID,
				Relation: "parent",
			}
			_ = guardianshipService.AddGuardian(ctx, dto)
		}(i)
	}

	close(start)
	wg.Wait()

	// Assert - 查询数据库中该儿童的监护人数量，期望为 1（防止重复创建）
	results, err := queryService.ListGuardiansByChildID(ctx, childResult.ID)
	require.NoError(t, err)

	// 记录数量
	t.Logf("concurrent add results count: %d", len(results))

	// 现在数据库层已添加唯一约束，期望只有一条监护关系被成功持久化
	require.Equal(t, 1, len(results))
}

func TestGuardianshipQueryApplicationService_ListChildrenByUserID_ExcludesRevokedByDefault(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := uow.NewUnitOfWork(db)
	ctx := context.Background()

	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "赵十",
		Phone: "13800138110",
		Email: "zhao10@test.com",
	})
	require.NoError(t, err)

	childService := child.NewChildApplicationService(unitOfWork)
	childResult, err := childService.Register(ctx, child.RegisterChildDTO{
		Name:     "小舟",
		Gender:   1,
		Birthday: "2020-10-10",
	})
	require.NoError(t, err)

	guardianshipService := guardianship.NewGuardianshipApplicationService(unitOfWork)
	require.NoError(t, guardianshipService.AddGuardian(ctx, guardianship.AddGuardianDTO{
		UserID:   userResult.ID,
		ChildID:  childResult.ID,
		Relation: "parent",
	}))
	require.NoError(t, guardianshipService.RemoveGuardian(ctx, guardianship.RemoveGuardianDTO{
		UserID:  userResult.ID,
		ChildID: childResult.ID,
	}))

	queryService := guardianship.NewGuardianshipQueryApplicationService(unitOfWork)

	activeOnly, err := queryService.ListChildrenByUserID(ctx, userResult.ID)
	require.NoError(t, err)
	assert.Len(t, activeOnly, 0)

	withRevoked, err := queryService.ListChildrenByUserIDIncludingRevoked(ctx, userResult.ID)
	require.NoError(t, err)
	require.Len(t, withRevoked, 1)
	assert.NotEmpty(t, withRevoked[0].RevokedAt)
}
