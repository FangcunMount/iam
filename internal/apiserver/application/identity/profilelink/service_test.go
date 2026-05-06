package profilelink_test

import (
	"context"
	"strconv"
	"sync"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profile"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profilelink"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/testutil"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// ==================== Commands 测试 ====================

func TestCommands_Establish_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	// 先创建用户和档案
	userService := user.NewCreator(unitOfWork)
	userResult, err := userService.Create(ctx, user.CreateUserDTO{
		Name:  "张三",
		Phone: "13800138000",
		Email: "zhang3@test.com",
	})
	require.NoError(t, err)

	profileService := profile.NewCreator(unitOfWork)
	profileResult, err := profileService.Create(ctx, profile.CreateProfileDTO{
		Name:     "小明",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	profileLinkService := profilelink.NewCommands(unitOfWork)

	// Act - 添加档案关系
	dto := profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	}
	_, err = profileLinkService.Establish(ctx, dto)

	// Assert
	require.NoError(t, err)

	// 验证档案关系是否创建成功
	queryService := profilelink.NewDirectory(unitOfWork)
	hasProfileLink, err := queryService.IsLinked(ctx, userResult.ID, profileResult.ID)
	require.NoError(t, err)
	assert.True(t, hasProfileLink)
}

func TestCommands_Establish_DuplicateLink(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	// 先创建用户和档案
	userService := user.NewCreator(unitOfWork)
	userResult, err := userService.Create(ctx, user.CreateUserDTO{
		Name:  "李四",
		Phone: "13800138001",
		Email: "li4@test.com",
	})
	require.NoError(t, err)

	profileService := profile.NewCreator(unitOfWork)
	profileResult, err := profileService.Create(ctx, profile.CreateProfileDTO{
		Name:     "小红",
		Gender:   2,
		Birthday: "2019-05-20",
	})
	require.NoError(t, err)

	profileLinkService := profilelink.NewCommands(unitOfWork)

	// 第一次添加档案关系
	dto := profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	}
	_, err = profileLinkService.Establish(ctx, dto)
	require.NoError(t, err)

	// Act - 尝试重复添加相同的档案关系
	_, err = profileLinkService.Establish(ctx, dto)

	// Assert - 应该失败
	require.Error(t, err)
}

func TestCommands_Revoke_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	// 先创建用户、档案和档案关系
	userService := user.NewCreator(unitOfWork)
	userResult, err := userService.Create(ctx, user.CreateUserDTO{
		Name:  "王五",
		Phone: "13800138002",
		Email: "wang5@test.com",
	})
	require.NoError(t, err)

	profileService := profile.NewCreator(unitOfWork)
	profileResult, err := profileService.Create(ctx, profile.CreateProfileDTO{
		Name:     "小强",
		Gender:   1,
		Birthday: "2021-03-10",
	})
	require.NoError(t, err)

	profileLinkService := profilelink.NewCommands(unitOfWork)
	_, err = profileLinkService.Establish(ctx, profilelink.CreateProfileLinkDTO{
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
	_, err = profileLinkService.Revoke(ctx, dto)

	// Assert
	require.NoError(t, err)

	// 验证档案关系是否已移除
	queryService := profilelink.NewDirectory(unitOfWork)
	result, err := queryService.Get(ctx, userResult.ID, profileResult.ID)
	require.Error(t, err)
	assert.Nil(t, result)

	historical, err := queryService.GetIncludingRevoked(ctx, userResult.ID, profileResult.ID)
	require.NoError(t, err)
	require.NotNil(t, historical)
	assert.NotEmpty(t, historical.RevokedAt)

	hasProfileLink, err := queryService.IsLinked(ctx, userResult.ID, profileResult.ID)
	require.NoError(t, err)
	assert.False(t, hasProfileLink)
}

func TestCommands_Revoke_NotFound(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	profileLinkService := profilelink.NewCommands(unitOfWork)
	ctx := context.Background()

	// Act - 尝试移除不存在的档案关系
	dto := profilelink.RemoveProfileLinkDTO{
		UserID:    "999999999999999999",
		ProfileID: "888888888888888888",
	}
	_, err := profileLinkService.Revoke(ctx, dto)

	// Assert - 应该失败
	require.Error(t, err)
}

func TestDirectory_HasProfileLink_True(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	// 创建用户、档案和档案关系
	userService := user.NewCreator(unitOfWork)
	userResult, err := userService.Create(ctx, user.CreateUserDTO{
		Name:  "孙七",
		Phone: "13800138004",
		Email: "sun7@test.com",
	})
	require.NoError(t, err)

	profileService := profile.NewCreator(unitOfWork)
	profileResult, err := profileService.Create(ctx, profile.CreateProfileDTO{
		Name:     "小虎",
		Gender:   1,
		Birthday: "2020-08-20",
	})
	require.NoError(t, err)

	profileLinkService := profilelink.NewCommands(unitOfWork)
	_, err = profileLinkService.Establish(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)

	queryService := profilelink.NewDirectory(unitOfWork)

	// Act - 检查是否为关系用户
	hasProfileLink, err := queryService.IsLinked(ctx, userResult.ID, profileResult.ID)

	// Assert
	require.NoError(t, err)
	assert.True(t, hasProfileLink)
}

func TestDirectory_HasProfileLink_False(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	queryService := profilelink.NewDirectory(unitOfWork)
	ctx := context.Background()

	// Act - 检查不存在的档案关系
	hasProfileLink, err := queryService.IsLinked(ctx, "999999999999999999", "888888888888888888")

	// Assert
	require.NoError(t, err)
	assert.False(t, hasProfileLink)
}

func TestDirectory_GetByUserIDAndProfileID_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	// 创建用户、档案和档案关系
	userService := user.NewCreator(unitOfWork)
	userResult, err := userService.Create(ctx, user.CreateUserDTO{
		Name:  "周八",
		Phone: "13800138005",
		Email: "zhou8@test.com",
	})
	require.NoError(t, err)

	profileService := profile.NewCreator(unitOfWork)
	profileResult, err := profileService.Create(ctx, profile.CreateProfileDTO{
		Name:     "小龙",
		Gender:   1,
		Birthday: "2019-12-25",
	})
	require.NoError(t, err)

	profileLinkService := profilelink.NewCommands(unitOfWork)
	_, err = profileLinkService.Establish(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
		Relation:  "grandparent",
	})
	require.NoError(t, err)

	queryService := profilelink.NewDirectory(unitOfWork)

	// Act - 查询档案关系
	result, err := queryService.Get(ctx, userResult.ID, profileResult.ID)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, userResult.ID, result.UserID)
	assert.Equal(t, profileResult.ID, result.ProfileID)
	assert.Equal(t, "小龙", result.ProfileName)
}

func TestDirectory_GetByUserIDAndProfileID_NotFound(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	queryService := profilelink.NewDirectory(unitOfWork)
	ctx := context.Background()

	// Act - 查询不存在的档案关系
	result, err := queryService.Get(ctx, "999999999999999999", "888888888888888888")

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestDirectory_ListProfilesByUserID_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	// 创建用户
	userService := user.NewCreator(unitOfWork)
	userResult, err := userService.Create(ctx, user.CreateUserDTO{
		Name:  "吴九",
		Phone: "13800138006",
		Email: "wu9@test.com",
	})
	require.NoError(t, err)

	// 创建多个档案
	profileService := profile.NewCreator(unitOfWork)
	profile1, err := profileService.Create(ctx, profile.CreateProfileDTO{
		Name:     "大宝",
		Gender:   1,
		Birthday: "2018-01-01",
		IDCard:   "110101201801011112",
	})
	require.NoError(t, err)

	profile2, err := profileService.Create(ctx, profile.CreateProfileDTO{
		Name:     "二宝",
		Gender:   2,
		Birthday: "2020-06-01",
		IDCard:   "110101202006012225",
	})
	require.NoError(t, err)

	// 添加档案关系
	profileLinkService := profilelink.NewCommands(unitOfWork)
	_, err = profileLinkService.Establish(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profile1.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)

	_, err = profileLinkService.Establish(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profile2.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)

	queryService := profilelink.NewDirectory(unitOfWork)

	// Act - 列出用户的所有档案
	results, err := queryService.ListProfilesForUser(ctx, userResult.ID)

	// Assert
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestDirectory_ListProfileLinksByProfileID_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	// 创建多个用户 (设置唯一的email避免UNIQUE约束冲突)
	userService := user.NewCreator(unitOfWork)
	user1, err := userService.Create(ctx, user.CreateUserDTO{
		Name:  "爸爸",
		Phone: "13800138007",
		Email: "father@example.com", // 唯一email
	})
	require.NoError(t, err)

	// 设置唯一的 IDCard 避免 UNIQUE 约束冲突
	userProfileService := user.NewEditor(unitOfWork)
	err = userProfileService.UpdateIDCard(ctx, user1.ID, "320106198001011110")
	require.NoError(t, err)

	user2, err := userService.Create(ctx, user.CreateUserDTO{
		Name:  "妈妈",
		Phone: "13800138008",
		Email: "mother@example.com", // 唯一email
	})
	require.NoError(t, err)

	// 设置唯一的 IDCard 避免 UNIQUE 约束冲突
	err = userProfileService.UpdateIDCard(ctx, user2.ID, "320106198001012228")
	require.NoError(t, err)

	// 创建档案
	profileService := profile.NewCreator(unitOfWork)
	profileResult, err := profileService.Create(ctx, profile.CreateProfileDTO{
		Name:     "宝宝",
		Gender:   2,
		Birthday: "2021-01-01",
	})
	require.NoError(t, err)

	// 添加多个关系用户
	profileLinkService := profilelink.NewCommands(unitOfWork)
	_, err = profileLinkService.Establish(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    user1.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)

	_, err = profileLinkService.Establish(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    user2.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)

	queryService := profilelink.NewDirectory(unitOfWork)

	// Act - 列出档案的所有关系用户
	results, err := queryService.ListLinksForProfile(ctx, profileResult.ID)

	// Assert
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestCommands_Establish_ConcurrentPersistence_10(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	// 创建用户和档案（使用唯一 email 避免 UNIQUE 冲突）
	userService := user.NewCreator(unitOfWork)
	userResult, err := userService.Create(ctx, user.CreateUserDTO{
		Name:  "并发父亲",
		Phone: "13900000000",
		Email: "concurrent_father@example.com",
	})
	require.NoError(t, err)

	profileService := profile.NewCreator(unitOfWork)
	profileResult, err := profileService.Create(ctx, profile.CreateProfileDTO{
		Name:     "并发档案",
		Gender:   1,
		Birthday: "2020-02-02",
	})
	require.NoError(t, err)

	profileLinkService := profilelink.NewCommands(unitOfWork)
	queryService := profilelink.NewDirectory(unitOfWork)

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
			_, _ = profileLinkService.Establish(ctx, dto)
		}(i)
	}

	close(start)
	wg.Wait()

	// Assert - 查询数据库中该档案的关系用户数量，期望为 1（防止重复创建）
	results, err := queryService.ListLinksForProfile(ctx, profileResult.ID)
	require.NoError(t, err)

	// 记录数量
	t.Logf("concurrent add results count: %d", len(results))

	// 现在数据库层已添加唯一约束，期望只有一条档案关系被成功持久化
	require.Equal(t, 1, len(results))
}

func TestDirectory_ListProfilesByUserID_ExcludesRevokedByDefault(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	userService := user.NewCreator(unitOfWork)
	userResult, err := userService.Create(ctx, user.CreateUserDTO{
		Name:  "赵十",
		Phone: "13800138110",
		Email: "zhao10@test.com",
	})
	require.NoError(t, err)

	profileService := profile.NewCreator(unitOfWork)
	profileResult, err := profileService.Create(ctx, profile.CreateProfileDTO{
		Name:     "小舟",
		Gender:   1,
		Birthday: "2020-10-10",
	})
	require.NoError(t, err)

	profileLinkService := profilelink.NewCommands(unitOfWork)
	_, err = profileLinkService.Establish(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)
	_, err = profileLinkService.Revoke(ctx, profilelink.RemoveProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
	})
	require.NoError(t, err)

	queryService := profilelink.NewDirectory(unitOfWork)

	activeOnly, err := queryService.ListProfilesForUser(ctx, userResult.ID)
	require.NoError(t, err)
	assert.Empty(t, activeOnly)

	withRevoked, err := queryService.ListProfilesForUserIncludingRevoked(ctx, userResult.ID)
	require.NoError(t, err)
	require.Len(t, withRevoked, 1)
	assert.True(t, hasRevokedProfileLink(withRevoked))
}

func TestMyProfileLinks_GrantAndList(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	userService := user.NewCreator(unitOfWork)
	userResult, err := userService.Create(ctx, user.CreateUserDTO{
		Name:  "当前用户",
		Phone: "13800139101",
		Email: "current@example.com",
	})
	require.NoError(t, err)

	profileService := profile.NewCreator(unitOfWork)
	profileResult, err := profileService.Create(ctx, profile.CreateProfileDTO{
		Name:     "档案",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	accessService := profilelink.NewMyProfileLinks(unitOfWork)
	created, err := accessService.Grant(ctx, userResult.ID, profilelink.CreateProfileLinkDTO{
		ProfileID: profileResult.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, userResult.ID, created.UserID)
	assert.Equal(t, profileResult.ID, created.ProfileID)

	results, err := accessService.List(ctx, userResult.ID, profilelink.ListProfileLinksDTO{})
	require.NoError(t, err)
	require.Len(t, results, 1)
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

func TestMyProfileLinks_RejectsCrossUserGrantAndList(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	accessService := profilelink.NewMyProfileLinks(unitOfWork)

	created, err := accessService.Grant(ctx, "1", profilelink.CreateProfileLinkDTO{
		UserID:    "2",
		ProfileID: "3",
		Relation:  "parent",
	})
	require.Error(t, err)
	assert.Nil(t, created)
	assert.True(t, perrors.IsCode(err, code.ErrPermissionDenied))

	results, err := accessService.List(ctx, "1", profilelink.ListProfileLinksDTO{UserID: "2"})
	require.Error(t, err)
	assert.Nil(t, results)
	assert.True(t, perrors.IsCode(err, code.ErrPermissionDenied))
}

func TestMyProfileLinks_RevokeBySelectorReturnsRevokedResult(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	userService := user.NewCreator(unitOfWork)
	userResult, err := userService.Create(ctx, user.CreateUserDTO{
		Name:  "撤销用户",
		Phone: "13800139102",
		Email: "revoke@example.com",
	})
	require.NoError(t, err)

	profileService := profile.NewCreator(unitOfWork)
	profileResult, err := profileService.Create(ctx, profile.CreateProfileDTO{
		Name:     "档案",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	linkCommands := profilelink.NewCommands(unitOfWork)
	_, err = linkCommands.Establish(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)

	accessService := profilelink.NewMyProfileLinks(unitOfWork)
	revoked, err := accessService.Revoke(ctx, userResult.ID, profilelink.RevokeProfileLinkBySelectorDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
	})
	require.NoError(t, err)
	require.NotNil(t, revoked)
	assert.Equal(t, userResult.ID, revoked.UserID)
	assert.Equal(t, profileResult.ID, revoked.ProfileID)
	assert.NotEmpty(t, revoked.RevokedAt)
}

func TestMyProfileLinks_RevokeRejectsOtherUsersProfileLink(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	userService := user.NewCreator(unitOfWork)
	owner, err := userService.Create(ctx, user.CreateUserDTO{
		Name:  "关系所有人",
		Phone: "13800139103",
		Email: "owner@example.com",
	})
	require.NoError(t, err)
	other, err := userService.Create(ctx, user.CreateUserDTO{
		Name:  "其他用户",
		Phone: "13800139104",
		Email: "other-revoke@example.com",
	})
	require.NoError(t, err)

	profileService := profile.NewCreator(unitOfWork)
	profileResult, err := profileService.Create(ctx, profile.CreateProfileDTO{
		Name:     "档案",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	command := profilelink.NewCommands(unitOfWork)
	link, err := command.Establish(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    owner.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)

	accessService := profilelink.NewMyProfileLinks(unitOfWork)
	revoked, err := accessService.Revoke(ctx, other.ID, profilelink.RevokeProfileLinkBySelectorDTO{
		ProfileLinkID: strconv.FormatUint(link.ID, 10),
	})

	require.Error(t, err)
	assert.Nil(t, revoked)
	assert.True(t, perrors.IsCode(err, code.ErrPermissionDenied))

	stillActive, err := profilelink.NewDirectory(unitOfWork).Get(ctx, owner.ID, profileResult.ID)
	require.NoError(t, err)
	assert.Empty(t, stillActive.RevokedAt)
}
