package profile_test

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profile"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profilelink"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/testutil"
	identityuow "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ==================== MyProfiles 创建测试 ====================

func TestMyProfiles_Create_Success(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	appService := profile.NewMyProfiles(unitOfWork)
	ctx := context.Background()
	currentUser := createCurrentUser(t, ctx, unitOfWork, "创建用户", "13800139201", "create1@example.com")

	dto := profile.CreateProfileDTO{
		Name:     "小明",
		Gender:   1, // 1=男
		Birthday: "2020-01-15",
		Relation: "self",
	}

	result, err := appService.Create(ctx, mustID(t, currentUser.ID), dto)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Profile)
	require.NotNil(t, result.ProfileLink)
	assert.NotEmpty(t, result.Profile.ID)
	assert.Equal(t, dto.Name, result.Profile.Name)
	assert.Equal(t, uint8(1), result.Profile.Gender)
	assert.Equal(t, dto.Birthday, result.Profile.Birthday)
	assert.Equal(t, "self", result.ProfileLink.Relation)
}

func TestMyProfiles_Create_WithOptionalFields(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	appService := profile.NewMyProfiles(unitOfWork)
	ctx := context.Background()
	currentUser := createCurrentUser(t, ctx, unitOfWork, "创建用户2", "13800139202", "create2@example.com")

	dto := profile.CreateProfileDTO{
		Name:     "小红",
		Gender:   2, // 2=女
		Birthday: "2019-05-20",
		IDCard:   "110101201905201236",
		Relation: "parent",
	}

	result, err := appService.Create(ctx, mustID(t, currentUser.ID), dto)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Profile)
	require.NotNil(t, result.ProfileLink)
	assert.NotEmpty(t, result.Profile.ID)
	assert.Equal(t, dto.Name, result.Profile.Name)
	assert.Equal(t, uint8(2), result.Profile.Gender)
	assert.Equal(t, dto.Birthday, result.Profile.Birthday)
	assert.Equal(t, dto.IDCard, result.Profile.IDCard)
	assert.Equal(t, "parent", result.ProfileLink.Relation)
	linked, err := profilelink.NewDirectory(unitOfWork).IsLinked(ctx, mustID(t, currentUser.ID), mustID(t, result.Profile.ID))
	require.NoError(t, err)
	assert.True(t, linked, "CreateProfile must establish an active link consumable by HasProfileLink")
}

func TestMyProfiles_Create_EmptyName_ShouldFail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	appService := profile.NewMyProfiles(unitOfWork)
	ctx := context.Background()
	currentUser := createCurrentUser(t, ctx, unitOfWork, "创建用户3", "13800139203", "create3@example.com")

	dto := profile.CreateProfileDTO{
		Name:     "", // 空姓名
		Gender:   1,  // 1=男
		Birthday: "2020-01-15",
		Relation: "parent",
	}

	result, err := appService.Create(ctx, mustID(t, currentUser.ID), dto)

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestMyProfiles_Create_DuplicateIDCard(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	appService := profile.NewMyProfiles(unitOfWork)
	ctx := context.Background()
	currentUser := createCurrentUser(t, ctx, unitOfWork, "创建用户4", "13800139204", "create4@example.com")

	dto1 := profile.CreateProfileDTO{
		Name:     "小明",
		Gender:   1, // 1=男
		Birthday: "2020-01-15",
		IDCard:   "110101202001151234",
		Relation: "parent",
	}
	_, err := appService.Create(ctx, mustID(t, currentUser.ID), dto1)
	require.NoError(t, err)

	dto2 := profile.CreateProfileDTO{
		Name:     "小红",
		Gender:   2, // 2=女
		Birthday: "2020-01-15",
		IDCard:   "110101202001151234", // 重复身份证
		Relation: "parent",
	}
	result, err := appService.Create(ctx, mustID(t, currentUser.ID), dto2)

	require.Error(t, err)
	assert.Nil(t, result)
}

// ==================== Directory 测试 ====================

func TestDirectory_GetByID_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	createUseCase := testutil.NewProfileFixture(t, unitOfWork)
	ctx := context.Background()

	created, err := createUseCase.Create(ctx, profile.CreateProfileDTO{
		Name:     "小明",
		Gender:   1, // 1=男
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	queryService := profile.NewDirectory(unitOfWork)

	// Act - 查询档案
	result, err := queryService.GetByID(ctx, mustID(t, created.ID))

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
	result, err := queryService.GetByID(ctx, meta.FromUint64(999999999999999999))

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestDirectory_GetByIDCard_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	createUseCase := testutil.NewProfileFixture(t, unitOfWork)
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

	profileService := testutil.NewProfileFixture(t, unitOfWork)
	profileResult, err := profileService.Create(ctx, profile.CreateProfileDTO{
		Name:     "旧名",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	linkCommands := profilelink.NewCommands(unitOfWork)
	_, err = linkCommands.Establish(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    mustID(t, userResult.ID),
		ProfileID: mustID(t, profileResult.ID),
		Relation:  "parent",
	})
	require.NoError(t, err)

	accessService := profile.NewMyProfiles(unitOfWork)
	profiles, err := accessService.List(ctx, mustID(t, userResult.ID))
	require.NoError(t, err)
	require.Len(t, profiles, 1)
	assert.Equal(t, profileResult.ID, profiles[0].ID)

	found, err := accessService.Get(ctx, mustID(t, userResult.ID), mustID(t, profileResult.ID))
	require.NoError(t, err)
	assert.Equal(t, profileResult.ID, found.ID)

	newName := "新名"
	gender := uint8(2)
	birthday := "2020-02-20"
	updated, err := accessService.Patch(ctx, profile.PatchMyProfileDTO{
		UserID:    mustID(t, userResult.ID),
		ProfileID: mustID(t, profileResult.ID),
		LegalName: &newName,
		Gender:    &gender,
		Birthday:  &birthday,
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, newName, updated.Name)
	assert.Equal(t, gender, updated.Gender)
	assert.Equal(t, birthday, updated.Birthday)
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

	profileResult, err := testutil.NewProfileFixture(t, unitOfWork).Create(ctx, profile.CreateProfileDTO{
		Name:     "旧名",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	_, err = profilelink.NewCommands(unitOfWork).Establish(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    mustID(t, userResult.ID),
		ProfileID: mustID(t, profileResult.ID),
		Relation:  "parent",
	})
	require.NoError(t, err)

	require.NoError(t, db.Exec(`
CREATE TRIGGER fail_profile_measurement_update
BEFORE UPDATE ON profiles
WHEN NEW.name = '不应保存'
BEGIN
  SELECT RAISE(ABORT, 'forced profile update failure');
END;`).Error)

	newName := "不应保存"
	updated, err := profile.NewMyProfiles(unitOfWork).Patch(ctx, profile.PatchMyProfileDTO{
		UserID:    mustID(t, userResult.ID),
		ProfileID: mustID(t, profileResult.ID),
		LegalName: &newName,
	})
	require.Error(t, err)
	assert.Nil(t, updated)

	persisted, err := profile.NewDirectory(unitOfWork).GetByID(ctx, mustID(t, profileResult.ID))
	require.NoError(t, err)
	assert.Equal(t, profileResult.Name, persisted.Name)
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

	profileService := testutil.NewProfileFixture(t, unitOfWork)
	profileResult, err := profileService.Create(ctx, profile.CreateProfileDTO{
		Name:     "档案",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	linkCommands := profilelink.NewCommands(unitOfWork)
	_, err = linkCommands.Establish(ctx, profilelink.CreateProfileLinkDTO{
		UserID:    mustID(t, profileLinkUser.ID),
		ProfileID: mustID(t, profileResult.ID),
		Relation:  "parent",
	})
	require.NoError(t, err)

	accessService := profile.NewMyProfiles(unitOfWork)
	result, err := accessService.Get(ctx, mustID(t, other.ID), mustID(t, profileResult.ID))
	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, perrors.IsCode(err, code.ErrPermissionDenied))
}

func mustID(t *testing.T, raw string) meta.ID {
	t.Helper()
	id, err := meta.ParseID(raw)
	require.NoError(t, err)
	return id
}

func createCurrentUser(
	t *testing.T,
	ctx context.Context,
	unitOfWork identityuow.UnitOfWork,
	name string,
	phone string,
	email string,
) *user.UserResult {
	t.Helper()
	result, err := user.NewCreator(unitOfWork).Create(ctx, user.CreateUserDTO{
		Name:  name,
		Phone: phone,
		Email: email,
	})
	require.NoError(t, err)
	return result
}
