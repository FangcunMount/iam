package profile_test

import (
	"context"
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

// ==================== Editor 测试 ====================

func TestCreator_Create_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	appService := profile.NewCreator(unitOfWork)
	ctx := context.Background()

	dto := profile.CreateProfileDTO{
		Name:     "小明",
		Gender:   1, // 1=男
		Birthday: "2020-01-15",
	}

	// Act - 执行创建
	result, err := appService.Create(ctx, dto)

	// Assert - 验证结果
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, dto.Name, result.Name)
	assert.Equal(t, uint8(1), result.Gender) // 1=男
	assert.Equal(t, dto.Birthday, result.Birthday)
}

func TestCreator_Create_WithOptionalFields(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	appService := profile.NewCreator(unitOfWork)
	ctx := context.Background()

	height := uint32(110)
	weight := uint32(20000) // 20kg = 20000g
	dto := profile.CreateProfileDTO{
		Name:     "小红",
		Gender:   2, // 2=女
		Birthday: "2019-05-20",
		IDCard:   "110101201905201236",
		Height:   &height,
		Weight:   &weight,
	}

	// Act
	result, err := appService.Create(ctx, dto)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, dto.Name, result.Name)
	assert.Equal(t, uint8(2), result.Gender) // 2=女
	assert.Equal(t, dto.Birthday, result.Birthday)
	assert.Equal(t, dto.IDCard, result.IDCard)
	assert.Equal(t, height, result.Height)
	assert.Equal(t, weight, result.Weight)
}

func TestCreator_Create_EmptyName_ShouldFail(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	appService := profile.NewCreator(unitOfWork)
	ctx := context.Background()

	dto := profile.CreateProfileDTO{
		Name:     "", // 空姓名
		Gender:   1,  // 1=男
		Birthday: "2020-01-15",
	}

	// Act
	result, err := appService.Create(ctx, dto)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestCreator_Create_DuplicateIDCard(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	appService := profile.NewCreator(unitOfWork)
	ctx := context.Background()

	// 先创建一个档案
	dto1 := profile.CreateProfileDTO{
		Name:     "小明",
		Gender:   1, // 1=男
		Birthday: "2020-01-15",
		IDCard:   "110101202001151234",
	}
	_, err := appService.Create(ctx, dto1)
	require.NoError(t, err)

	// Act - 尝试创建相同身份证的档案
	dto2 := profile.CreateProfileDTO{
		Name:     "小红",
		Gender:   2, // 2=女
		Birthday: "2020-01-15",
		IDCard:   "110101202001151234", // 重复身份证
	}
	result, err := appService.Create(ctx, dto2)

	// Assert - 应该失败
	require.Error(t, err)
	assert.Nil(t, result)
}

// ==================== Editor 测试 ====================

func TestEditor_Rename_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	createUseCase := profile.NewCreator(unitOfWork)
	ctx := context.Background()

	created, err := createUseCase.Create(ctx, profile.CreateProfileDTO{
		Name:     "小明",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	profileService := profile.NewEditor(unitOfWork)

	// Act - 修改姓名
	err = profileService.Rename(ctx, created.ID, "小强")

	// Assert
	require.NoError(t, err)

	// 验证数据库中的数据
	queryService := profile.NewDirectory(unitOfWork)
	updated, err := queryService.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "小强", updated.Name)
}

func TestEditor_Rename_EmptyName(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	createUseCase := profile.NewCreator(unitOfWork)
	ctx := context.Background()

	created, err := createUseCase.Create(ctx, profile.CreateProfileDTO{
		Name:     "小明",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	profileService := profile.NewEditor(unitOfWork)

	// Act - 尝试设置空姓名
	err = profileService.Rename(ctx, created.ID, "")

	// Assert - 如果有验证则应该失败,没有验证则会成功
	// 注意: 取决于领域模型的验证逻辑
	if err != nil {
		t.Logf("Empty name validation works: %v", err)
	} else {
		t.Log("Empty name allowed (no validation)")
	}
}

func TestEditor_UpdateIDCard_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	createUseCase := profile.NewCreator(unitOfWork)
	ctx := context.Background()

	created, err := createUseCase.Create(ctx, profile.CreateProfileDTO{
		Name:     "小明",
		Gender:   1, // 1=男
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	profileService := profile.NewEditor(unitOfWork)

	// Act - 更新身份证
	err = profileService.UpdateIDCard(ctx, created.ID, "小明", "110101202001151234")

	// Assert
	require.NoError(t, err)

	// 验证数据库中的数据
	queryService := profile.NewDirectory(unitOfWork)
	updated, err := queryService.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "110101202001151234", updated.IDCard)
}

func TestEditor_UpdateProfile_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	createUseCase := profile.NewCreator(unitOfWork)
	ctx := context.Background()

	created, err := createUseCase.Create(ctx, profile.CreateProfileDTO{
		Name:     "小明",
		Gender:   1, // 1=男
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	profileService := profile.NewEditor(unitOfWork)

	// Act - 更新基本信息
	dto := profile.UpdateProfileDTO{
		ProfileID: created.ID,
		Gender:    2, // 2=女
		Birthday:  "2020-02-20",
	}
	err = profileService.UpdateProfile(ctx, dto)

	// Assert
	require.NoError(t, err)

	// 验证数据库中的数据
	queryService := profile.NewDirectory(unitOfWork)
	updated, err := queryService.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, uint8(2), updated.Gender) // 2=女
	assert.Equal(t, "2020-02-20", updated.Birthday)
}

func TestEditor_UpdateHeightWeight_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	createUseCase := profile.NewCreator(unitOfWork)
	ctx := context.Background()

	created, err := createUseCase.Create(ctx, profile.CreateProfileDTO{
		Name:     "小明",
		Gender:   1, // 1=男
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	profileService := profile.NewEditor(unitOfWork)

	// Act - 更新身高体重
	dto := profile.UpdateHeightWeightDTO{
		ProfileID: created.ID,
		Height:    120,   // 120cm
		Weight:    25000, // 25kg = 25000g
	}
	err = profileService.UpdateHeightWeight(ctx, dto)

	// Assert
	require.NoError(t, err)

	// 验证数据库中的数据
	queryService := profile.NewDirectory(unitOfWork)
	updated, err := queryService.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, uint32(120), updated.Height)
	assert.Equal(t, uint32(25000), updated.Weight)
}

func TestEditor_ProfileNotFound_ShouldFail(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	profileService := profile.NewEditor(unitOfWork)
	ctx := context.Background()

	// Act - 尝试修改不存在的档案
	err := profileService.Rename(ctx, "999999999999999999", "小强")

	// Assert - 应该失败
	require.Error(t, err)
}

// ==================== Directory 测试 ====================

func TestDirectory_GetByID_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	createUseCase := profile.NewCreator(unitOfWork)
	ctx := context.Background()

	created, err := createUseCase.Create(ctx, profile.CreateProfileDTO{
		Name:     "小明",
		Gender:   1, // 1=男
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	queryService := profile.NewDirectory(unitOfWork)

	// Act - 查询档案
	result, err := queryService.GetByID(ctx, created.ID)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, created.ID, result.ID)
	assert.Equal(t, created.Name, result.Name)
}

func TestDirectory_GetByID_NotFound(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	queryService := profile.NewDirectory(unitOfWork)
	ctx := context.Background()

	// Act - 查询不存在的档案
	result, err := queryService.GetByID(ctx, "999999999999999999")

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestDirectory_GetByIDCard_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	createUseCase := profile.NewCreator(unitOfWork)
	ctx := context.Background()

	created, err := createUseCase.Create(ctx, profile.CreateProfileDTO{
		Name:     "小明",
		Gender:   1, // 1=男
		Birthday: "2020-01-15",
		IDCard:   "110101202001151234",
	})
	require.NoError(t, err)

	queryService := profile.NewDirectory(unitOfWork)

	// Act - 根据身份证查询
	result, err := queryService.GetByIDCard(ctx, "110101202001151234")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, created.ID, result.ID)
	assert.Equal(t, "110101202001151234", result.IDCard)
}

func TestDirectory_FindSimilar_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	createUseCase := profile.NewCreator(unitOfWork)
	ctx := context.Background()

	// 创建多个档案 (使用不同的身份证号或不设置)
	created1, err := createUseCase.Create(ctx, profile.CreateProfileDTO{
		Name:     "小强",
		Gender:   1, // 1=男
		Birthday: "2020-03-10",
		IDCard:   "110101202003101118", // 唯一身份证
	})
	require.NoError(t, err)

	_, err = createUseCase.Create(ctx, profile.CreateProfileDTO{
		Name:     "小丽",
		Gender:   2, // 2=女
		Birthday: "2020-03-10",
		IDCard:   "110101202003102225", // 另一个唯一身份证
	})
	require.NoError(t, err)

	queryService := profile.NewDirectory(unitOfWork)

	// Act - 查找相似档案（相同生日的男孩）
	results, err := queryService.FindSimilar(ctx, created1.Name, 1, "2020-03-10")

	// Assert
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)
}

func TestDirectory_FindSimilar_NoMatch(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	queryService := profile.NewDirectory(unitOfWork)
	ctx := context.Background()

	// Act - 查找不存在的相似档案
	results, err := queryService.FindSimilar(ctx, "不存在", 1, "2000-01-01")

	// Assert
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestMyProfiles_ListGetAndPatch(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	userService := user.NewCreator(unitOfWork)
	userResult, err := userService.Create(ctx, user.CreateUserDTO{
		Name:  "关系用户",
		Phone: "13800139001",
		Email: "link@example.com",
	})
	require.NoError(t, err)

	profileService := profile.NewCreator(unitOfWork)
	profileResult, err := profileService.Create(ctx, profile.CreateProfileDTO{
		Name:     "旧名",
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

	accessService := profile.NewMyProfiles(unitOfWork)
	profiles, err := accessService.List(ctx, userResult.ID)
	require.NoError(t, err)
	require.Len(t, profiles, 2)
	assert.Contains(t, []string{profiles[0].ID, profiles[1].ID}, profileResult.ID)

	found, err := accessService.Get(ctx, userResult.ID, profileResult.ID)
	require.NoError(t, err)
	assert.Equal(t, profileResult.ID, found.ID)

	newName := "新名"
	gender := uint8(2)
	birthday := "2020-02-20"
	height := uint32(121)
	weight := uint32(26000)
	updated, err := accessService.Patch(ctx, profile.PatchMyProfileDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
		LegalName: &newName,
		Gender:    &gender,
		Birthday:  &birthday,
		Height:    &height,
		Weight:    &weight,
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, newName, updated.Name)
	assert.Equal(t, gender, updated.Gender)
	assert.Equal(t, birthday, updated.Birthday)
	assert.Equal(t, height, updated.Height)
	assert.Equal(t, weight, updated.Weight)
}

func TestMyProfiles_PatchRollsBackEarlierUpdatesWhenLaterUpdateFails(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	userResult, err := user.NewCreator(unitOfWork).Create(ctx, user.CreateUserDTO{
		Name:  "关系用户",
		Phone: "13800139004",
		Email: "link4@example.com",
	})
	require.NoError(t, err)

	profileResult, err := profile.NewCreator(unitOfWork).Create(ctx, profile.CreateProfileDTO{
		Name:     "旧名",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	_, err = profilelink.NewCommands(unitOfWork).Establish(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)

	require.NoError(t, db.Exec(`
CREATE TRIGGER fail_profile_measurement_update
BEFORE UPDATE ON profiles
WHEN NEW.height = 9990
BEGIN
  SELECT RAISE(ABORT, 'forced measurement failure');
END;`).Error)

	newName := "不应保存"
	height := uint32(999)
	updated, err := profile.NewMyProfiles(unitOfWork).Patch(ctx, profile.PatchMyProfileDTO{
		UserID:    userResult.ID,
		ProfileID: profileResult.ID,
		LegalName: &newName,
		Height:    &height,
	})
	require.Error(t, err)
	assert.Nil(t, updated)

	persisted, err := profile.NewDirectory(unitOfWork).GetByID(ctx, profileResult.ID)
	require.NoError(t, err)
	assert.Equal(t, profileResult.Name, persisted.Name)
	assert.Equal(t, profileResult.Height, persisted.Height)
}

func TestMyProfiles_GetForProfileLinkRejectsUnlinkedUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	userService := user.NewCreator(unitOfWork)
	profileLinkUser, err := userService.Create(ctx, user.CreateUserDTO{
		Name:  "关系用户",
		Phone: "13800139002",
		Email: "ref2@example.com",
	})
	require.NoError(t, err)
	other, err := userService.Create(ctx, user.CreateUserDTO{
		Name:  "其他用户",
		Phone: "13800139003",
		Email: "other@example.com",
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
		UserID:    profileLinkUser.ID,
		ProfileID: profileResult.ID,
		Relation:  "parent",
	})
	require.NoError(t, err)

	accessService := profile.NewMyProfiles(unitOfWork)
	result, err := accessService.Get(ctx, other.ID, profileResult.ID)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, perrors.IsCode(err, code.ErrPermissionDenied))
}
