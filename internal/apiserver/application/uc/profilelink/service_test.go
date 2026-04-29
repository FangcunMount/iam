package profilelink_test

import (
	"context"
	"sync"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/internal/apiserver/application/uc/profile"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/profilelink"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/testutil"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// ==================== ProfileLinkApplicationService 测试 ====================

func TestProfileLinkApplicationService_CreateProfileLink_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	// 先创建用户和档案
	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "张三",
		Phone: "13800138000",
		Email: "zhang3@test.com",
	})
	require.NoError(t, err)

	profileService := profile.NewProfileApplicationService(unitOfWork)
	profileResult, err := profileService.Register(ctx, profile.RegisterProfileDTO{
		Name:     "小明",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	profileLinkService := profilelink.NewProfileLinkApplicationService(unitOfWork)

	// Act - 添加档案关系
	dto := profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	}
	err = profileLinkService.CreateProfileLink(ctx, dto)

	// Assert
	require.NoError(t, err)

	// 验证档案关系是否创建成功
	queryService := profilelink.NewProfileLinkQueryApplicationService(unitOfWork)
	hasProfileLink, err := queryService.HasProfileLink(ctx, userResult.ID, profileResult.ID)
	require.NoError(t, err)
	assert.True(t, hasProfileLink)
}

func TestProfileLinkApplicationService_CreateProfileLink_DuplicateRef(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	// 先创建用户和档案
	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "李四",
		Phone: "13800138001",
		Email: "li4@test.com",
	})
	require.NoError(t, err)

	profileService := profile.NewProfileApplicationService(unitOfWork)
	profileResult, err := profileService.Register(ctx, profile.RegisterProfileDTO{
		Name:     "小红",
		Gender:   2,
		Birthday: "2019-05-20",
	})
	require.NoError(t, err)

	profileLinkService := profilelink.NewProfileLinkApplicationService(unitOfWork)

	// 第一次添加档案关系
	dto := profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	}
	err = profileLinkService.CreateProfileLink(ctx, dto)
	require.NoError(t, err)

	// Act - 尝试重复添加相同的档案关系
	err = profileLinkService.CreateProfileLink(ctx, dto)

	// Assert - 应该失败
	require.Error(t, err)
}

func TestProfileLinkApplicationService_RemoveProfileLink_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	// 先创建用户、档案和档案关系
	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "王五",
		Phone: "13800138002",
		Email: "wang5@test.com",
	})
	require.NoError(t, err)

	profileService := profile.NewProfileApplicationService(unitOfWork)
	profileResult, err := profileService.Register(ctx, profile.RegisterProfileDTO{
		Name:     "小强",
		Gender:   1,
		Birthday: "2021-03-10",
	})
	require.NoError(t, err)

	profileLinkService := profilelink.NewProfileLinkApplicationService(unitOfWork)
	err = profileLinkService.CreateProfileLink(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)

	// Act - 移除档案关系
	dto := profilelink.RemoveProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
	}
	err = profileLinkService.RemoveProfileLink(ctx, dto)

	// Assert
	require.NoError(t, err)

	// 验证档案关系是否已移除
	queryService := profilelink.NewProfileLinkQueryApplicationService(unitOfWork)
	result, err := queryService.GetByUserIDAndProfileID(ctx, userResult.ID, profileResult.ID)
	require.Error(t, err)
	assert.Nil(t, result)

	historical, err := queryService.GetByUserIDAndProfileIDIncludingRevoked(ctx, userResult.ID, profileResult.ID)
	require.NoError(t, err)
	require.NotNil(t, historical)
	assert.NotEmpty(t, historical.RevokedAt)

	hasProfileLink, err := queryService.HasProfileLink(ctx, userResult.ID, profileResult.ID)
	require.NoError(t, err)
	assert.False(t, hasProfileLink)
}

func TestProfileLinkApplicationService_RemoveProfileLink_NotFound(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	profileLinkService := profilelink.NewProfileLinkApplicationService(unitOfWork)
	ctx := context.Background()

	// Act - 尝试移除不存在的档案关系
	dto := profilelink.RemoveProfileLinkDTO{
		UserID:    "999999999999999999",
		ProfileID: "888888888888888888",
	}
	err := profileLinkService.RemoveProfileLink(ctx, dto)

	// Assert - 应该失败
	require.Error(t, err)
}

func TestProfileLinkQueryApplicationService_HasProfileLink_True(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	// 创建用户、档案和档案关系
	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "孙七",
		Phone: "13800138004",
		Email: "sun7@test.com",
	})
	require.NoError(t, err)

	profileService := profile.NewProfileApplicationService(unitOfWork)
	profileResult, err := profileService.Register(ctx, profile.RegisterProfileDTO{
		Name:     "小虎",
		Gender:   1,
		Birthday: "2020-08-20",
	})
	require.NoError(t, err)

	profileLinkService := profilelink.NewProfileLinkApplicationService(unitOfWork)
	err = profileLinkService.CreateProfileLink(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)

	queryService := profilelink.NewProfileLinkQueryApplicationService(unitOfWork)

	// Act - 检查是否为关系用户
	hasProfileLink, err := queryService.HasProfileLink(ctx, userResult.ID, profileResult.ID)

	// Assert
	require.NoError(t, err)
	assert.True(t, hasProfileLink)
}

func TestProfileLinkQueryApplicationService_HasProfileLink_False(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	queryService := profilelink.NewProfileLinkQueryApplicationService(unitOfWork)
	ctx := context.Background()

	// Act - 检查不存在的档案关系
	hasProfileLink, err := queryService.HasProfileLink(ctx, "999999999999999999", "888888888888888888")

	// Assert
	require.NoError(t, err)
	assert.False(t, hasProfileLink)
}

func TestProfileLinkQueryApplicationService_GetByUserIDAndProfileID_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	// 创建用户、档案和档案关系
	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "周八",
		Phone: "13800138005",
		Email: "zhou8@test.com",
	})
	require.NoError(t, err)

	profileService := profile.NewProfileApplicationService(unitOfWork)
	profileResult, err := profileService.Register(ctx, profile.RegisterProfileDTO{
		Name:     "小龙",
		Gender:   1,
		Birthday: "2019-12-25",
	})
	require.NoError(t, err)

	profileLinkService := profilelink.NewProfileLinkApplicationService(unitOfWork)
	err = profileLinkService.CreateProfileLink(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
		Relation:  "grandparent",
	})
	require.NoError(t, err)

	queryService := profilelink.NewProfileLinkQueryApplicationService(unitOfWork)

	// Act - 查询档案关系
	result, err := queryService.GetByUserIDAndProfileID(ctx, userResult.ID, profileResult.ID)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, userResult.ID, result.UserID)
	assert.Equal(t, profileResult.ID, result.ProfileID)
	assert.Equal(t, "小龙", result.ProfileName)
}

func TestProfileLinkQueryApplicationService_GetByUserIDAndProfileID_NotFound(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	queryService := profilelink.NewProfileLinkQueryApplicationService(unitOfWork)
	ctx := context.Background()

	// Act - 查询不存在的档案关系
	result, err := queryService.GetByUserIDAndProfileID(ctx, "999999999999999999", "888888888888888888")

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestProfileLinkQueryApplicationService_ListProfilesByUserID_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	// 创建用户
	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "吴九",
		Phone: "13800138006",
		Email: "wu9@test.com",
	})
	require.NoError(t, err)

	// 创建多个档案
	profileService := profile.NewProfileApplicationService(unitOfWork)
	profile1, err := profileService.Register(ctx, profile.RegisterProfileDTO{
		Name:     "大宝",
		Gender:   1,
		Birthday: "2018-01-01",
		IDCard:   "110101201801011112",
	})
	require.NoError(t, err)

	profile2, err := profileService.Register(ctx, profile.RegisterProfileDTO{
		Name:     "二宝",
		Gender:   2,
		Birthday: "2020-06-01",
		IDCard:   "110101202006012225",
	})
	require.NoError(t, err)

	// 添加档案关系
	profileLinkService := profilelink.NewProfileLinkApplicationService(unitOfWork)
	err = profileLinkService.CreateProfileLink(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profile1.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)

	err = profileLinkService.CreateProfileLink(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profile2.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)

	queryService := profilelink.NewProfileLinkQueryApplicationService(unitOfWork)

	// Act - 列出用户的所有档案
	results, err := queryService.ListProfilesByUserID(ctx, userResult.ID)

	// Assert
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestProfileLinkQueryApplicationService_ListProfileLinksByProfileID_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
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
	userProfileService := user.NewUserProfileApplicationService(unitOfWork)
	err = userProfileService.UpdateIDCard(ctx, user1.ID, "320106198001011110")
	require.NoError(t, err)

	user2, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "妈妈",
		Phone: "13800138008",
		Email: "mother@example.com", // 唯一email
	})
	require.NoError(t, err)

	// 设置唯一的 IDCard 避免 UNIQUE 约束冲突
	err = userProfileService.UpdateIDCard(ctx, user2.ID, "320106198001012228")
	require.NoError(t, err)

	// 创建档案
	profileService := profile.NewProfileApplicationService(unitOfWork)
	profileResult, err := profileService.Register(ctx, profile.RegisterProfileDTO{
		Name:     "宝宝",
		Gender:   2,
		Birthday: "2021-01-01",
	})
	require.NoError(t, err)

	// 添加多个关系用户
	profileLinkService := profilelink.NewProfileLinkApplicationService(unitOfWork)
	err = profileLinkService.CreateProfileLink(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    user1.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)

	err = profileLinkService.CreateProfileLink(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    user2.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)

	queryService := profilelink.NewProfileLinkQueryApplicationService(unitOfWork)

	// Act - 列出档案的所有关系用户
	results, err := queryService.ListProfileLinksByProfileID(ctx, profileResult.ID)

	// Assert
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestProfileLinkApplicationService_CreateProfileLink_ConcurrentPersistence_10(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	// 创建用户和档案（使用唯一 email 避免 UNIQUE 冲突）
	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "并发父亲",
		Phone: "13900000000",
		Email: "concurrent_father@example.com",
	})
	require.NoError(t, err)

	profileService := profile.NewProfileApplicationService(unitOfWork)
	profileResult, err := profileService.Register(ctx, profile.RegisterProfileDTO{
		Name:     "并发档案",
		Gender:   1,
		Birthday: "2020-02-02",
	})
	require.NoError(t, err)

	profileLinkService := profilelink.NewProfileLinkApplicationService(unitOfWork)
	queryService := profilelink.NewProfileLinkQueryApplicationService(unitOfWork)

	// Act - 并发发起 10 个添加关系请求
	const N = 10
	var wg sync.WaitGroup
	wg.Add(N)

	// 为了提高并发命中率，让 goroutine 等待同一个开始信号
	start := make(chan struct{})

	for i := 0; i < N; i++ {
		go func(idx int) {
			defer wg.Done()
			<-start
			dto := profilelink.CreateProfileLinkDTO{
				UserID:    userResult.ID,
				ProfileID: profileResult.ID,
				Relation:  "parent",
			}
			_ = profileLinkService.CreateProfileLink(ctx, dto)
		}(i)
	}

	close(start)
	wg.Wait()

	// Assert - 查询数据库中该档案的关系用户数量，期望为 1（防止重复创建）
	results, err := queryService.ListProfileLinksByProfileID(ctx, profileResult.ID)
	require.NoError(t, err)

	// 记录数量
	t.Logf("concurrent add results count: %d", len(results))

	// 现在数据库层已添加唯一约束，期望只有一条档案关系被成功持久化
	require.Equal(t, 1, len(results))
}

func TestProfileLinkQueryApplicationService_ListProfilesByUserID_ExcludesRevokedByDefault(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "赵十",
		Phone: "13800138110",
		Email: "zhao10@test.com",
	})
	require.NoError(t, err)

	profileService := profile.NewProfileApplicationService(unitOfWork)
	profileResult, err := profileService.Register(ctx, profile.RegisterProfileDTO{
		Name:     "小舟",
		Gender:   1,
		Birthday: "2020-10-10",
	})
	require.NoError(t, err)

	profileLinkService := profilelink.NewProfileLinkApplicationService(unitOfWork)
	require.NoError(t, profileLinkService.CreateProfileLink(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	}))
	require.NoError(t, profileLinkService.RemoveProfileLink(ctx, profilelink.RemoveProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
	}))

	queryService := profilelink.NewProfileLinkQueryApplicationService(unitOfWork)

	activeOnly, err := queryService.ListProfilesByUserID(ctx, userResult.ID)
	require.NoError(t, err)
	assert.Len(t, activeOnly, 1)

	withRevoked, err := queryService.ListProfilesByUserIDIncludingRevoked(ctx, userResult.ID)
	require.NoError(t, err)
	require.Len(t, withRevoked, 2)
	assert.True(t, hasRevokedProfileLink(withRevoked))
}

func TestProfileLinkAccessApplicationService_GrantAndListForCurrentUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "当前用户",
		Phone: "13800139101",
		Email: "current@example.com",
	})
	require.NoError(t, err)

	profileService := profile.NewProfileApplicationService(unitOfWork)
	profileResult, err := profileService.Register(ctx, profile.RegisterProfileDTO{
		Name:     "档案",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	accessService := profilelink.NewProfileLinkAccessApplicationService(unitOfWork)
	created, err := accessService.GrantForCurrentUser(ctx, userResult.ID, profilelink.CreateProfileLinkDTO{
		ProfileID: profileResult.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, userResult.ID, created.UserID)
	assert.Equal(t, profileResult.ID, created.ProfileID)

	results, err := accessService.ListForCurrentUser(ctx, userResult.ID, profilelink.ListProfileLinksDTO{})
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.True(t, hasProfileLink(results, profileResult.ID))
}

func hasRevokedProfileLink(results []*profilelink.ProfileLinkResult) bool {
	for _, result := range results {
		if result != nil && result.RevokedAt != "" {
			return true
		}
	}
	return false
}

func hasProfileLink(results []*profilelink.ProfileLinkResult, profileID string) bool {
	for _, result := range results {
		if result != nil && result.ProfileID == profileID {
			return true
		}
	}
	return false
}

func TestProfileLinkAccessApplicationService_RejectsCrossUserGrantAndList(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	accessService := profilelink.NewProfileLinkAccessApplicationService(unitOfWork)

	created, err := accessService.GrantForCurrentUser(ctx, "1", profilelink.CreateProfileLinkDTO{
		UserID:    "2",
		ProfileID: "3",
		Relation:  "parent",
	})
	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, perrors.IsCode(err, code.ErrPermissionDenied))

	results, err := accessService.ListForCurrentUser(ctx, "1", profilelink.ListProfileLinksDTO{UserID: "2"})
	require.Error(t, err)
	assert.Nil(t, results)
	assert.True(t, perrors.IsCode(err, code.ErrPermissionDenied))
}

func TestProfileLinkAccessApplicationService_RevokeBySelectorReturnsRevokedResult(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "撤销用户",
		Phone: "13800139102",
		Email: "revoke@example.com",
	})
	require.NoError(t, err)

	profileService := profile.NewProfileApplicationService(unitOfWork)
	profileResult, err := profileService.Register(ctx, profile.RegisterProfileDTO{
		Name:     "档案",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	guardService := profilelink.NewProfileLinkApplicationService(unitOfWork)
	require.NoError(t, guardService.CreateProfileLink(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	}))

	accessService := profilelink.NewProfileLinkAccessApplicationService(unitOfWork)
	revoked, err := accessService.RevokeBySelector(ctx, profilelink.RevokeProfileLinkBySelectorDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
	})
	require.NoError(t, err)
	require.NotNil(t, revoked)
	assert.Equal(t, userResult.ID, revoked.UserID)
	assert.Equal(t, profileResult.ID, revoked.ProfileID)
	assert.NotEmpty(t, revoked.RevokedAt)
}
