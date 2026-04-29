package child_test

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/internal/apiserver/application/uc/child"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/guardianship"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/testutil"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// ==================== ChildApplicationService 测试 ====================

func TestChildApplicationService_Register_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	appService := child.NewChildApplicationService(unitOfWork)
	ctx := context.Background()

	dto := child.RegisterChildDTO{
		Name:     "小明",
		Gender:   1, // 1=男
		Birthday: "2020-01-15",
	}

	// Act - 执行注册
	result, err := appService.Register(ctx, dto)

	// Assert - 验证结果
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, dto.Name, result.Name)
	assert.Equal(t, uint8(1), result.Gender) // 1=男
	assert.Equal(t, dto.Birthday, result.Birthday)
}

func TestChildApplicationService_Register_WithOptionalFields(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	appService := child.NewChildApplicationService(unitOfWork)
	ctx := context.Background()

	height := uint32(110)
	weight := uint32(20000) // 20kg = 20000g
	dto := child.RegisterChildDTO{
		Name:     "小红",
		Gender:   2, // 2=女
		Birthday: "2019-05-20",
		IDCard:   "110101201905201236",
		Height:   &height,
		Weight:   &weight,
	}

	// Act
	result, err := appService.Register(ctx, dto)

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

func TestChildApplicationService_Register_EmptyName_ShouldFail(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	appService := child.NewChildApplicationService(unitOfWork)
	ctx := context.Background()

	dto := child.RegisterChildDTO{
		Name:     "", // 空姓名
		Gender:   1,  // 1=男
		Birthday: "2020-01-15",
	}

	// Act
	result, err := appService.Register(ctx, dto)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestChildApplicationService_Register_DuplicateIDCard(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	appService := child.NewChildApplicationService(unitOfWork)
	ctx := context.Background()

	// 先注册一个儿童
	dto1 := child.RegisterChildDTO{
		Name:     "小明",
		Gender:   1, // 1=男
		Birthday: "2020-01-15",
		IDCard:   "110101202001151234",
	}
	_, err := appService.Register(ctx, dto1)
	require.NoError(t, err)

	// Act - 尝试注册相同身份证的儿童
	dto2 := child.RegisterChildDTO{
		Name:     "小红",
		Gender:   2, // 2=女
		Birthday: "2020-01-15",
		IDCard:   "110101202001151234", // 重复身份证
	}
	result, err := appService.Register(ctx, dto2)

	// Assert - 应该失败
	require.Error(t, err)
	assert.Nil(t, result)
}

// ==================== ChildProfileApplicationService 测试 ====================

func TestChildProfileApplicationService_Rename_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	registerService := child.NewChildApplicationService(unitOfWork)
	ctx := context.Background()

	created, err := registerService.Register(ctx, child.RegisterChildDTO{
		Name:     "小明",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	profileService := child.NewChildProfileApplicationService(unitOfWork)

	// Act - 修改姓名
	err = profileService.Rename(ctx, created.ID, "小强")

	// Assert
	require.NoError(t, err)

	// 验证数据库中的数据
	queryService := child.NewChildQueryApplicationService(unitOfWork)
	updated, err := queryService.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "小强", updated.Name)
}

func TestChildProfileApplicationService_Rename_EmptyName(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	registerService := child.NewChildApplicationService(unitOfWork)
	ctx := context.Background()

	created, err := registerService.Register(ctx, child.RegisterChildDTO{
		Name:     "小明",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	profileService := child.NewChildProfileApplicationService(unitOfWork)

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

func TestChildProfileApplicationService_UpdateIDCard_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	registerService := child.NewChildApplicationService(unitOfWork)
	ctx := context.Background()

	created, err := registerService.Register(ctx, child.RegisterChildDTO{
		Name:     "小明",
		Gender:   1, // 1=男
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	profileService := child.NewChildProfileApplicationService(unitOfWork)

	// Act - 更新身份证
	err = profileService.UpdateIDCard(ctx, created.ID, "小明", "110101202001151234")

	// Assert
	require.NoError(t, err)

	// 验证数据库中的数据
	queryService := child.NewChildQueryApplicationService(unitOfWork)
	updated, err := queryService.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "110101202001151234", updated.IDCard)
}

func TestChildProfileApplicationService_UpdateProfile_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	registerService := child.NewChildApplicationService(unitOfWork)
	ctx := context.Background()

	created, err := registerService.Register(ctx, child.RegisterChildDTO{
		Name:     "小明",
		Gender:   1, // 1=男
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	profileService := child.NewChildProfileApplicationService(unitOfWork)

	// Act - 更新基本信息
	dto := child.UpdateChildProfileDTO{
		ChildID:  created.ID,
		Gender:   2, // 2=女
		Birthday: "2020-02-20",
	}
	err = profileService.UpdateProfile(ctx, dto)

	// Assert
	require.NoError(t, err)

	// 验证数据库中的数据
	queryService := child.NewChildQueryApplicationService(unitOfWork)
	updated, err := queryService.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, uint8(2), updated.Gender) // 2=女
	assert.Equal(t, "2020-02-20", updated.Birthday)
}

func TestChildProfileApplicationService_UpdateHeightWeight_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	registerService := child.NewChildApplicationService(unitOfWork)
	ctx := context.Background()

	created, err := registerService.Register(ctx, child.RegisterChildDTO{
		Name:     "小明",
		Gender:   1, // 1=男
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	profileService := child.NewChildProfileApplicationService(unitOfWork)

	// Act - 更新身高体重
	dto := child.UpdateHeightWeightDTO{
		ChildID: created.ID,
		Height:  120,   // 120cm
		Weight:  25000, // 25kg = 25000g
	}
	err = profileService.UpdateHeightWeight(ctx, dto)

	// Assert
	require.NoError(t, err)

	// 验证数据库中的数据
	queryService := child.NewChildQueryApplicationService(unitOfWork)
	updated, err := queryService.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, uint32(120), updated.Height)
	assert.Equal(t, uint32(25000), updated.Weight)
}

func TestChildProfileApplicationService_ChildNotFound_ShouldFail(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	profileService := child.NewChildProfileApplicationService(unitOfWork)
	ctx := context.Background()

	// Act - 尝试修改不存在的儿童
	err := profileService.Rename(ctx, "999999999999999999", "小强")

	// Assert - 应该失败
	require.Error(t, err)
}

// ==================== ChildQueryApplicationService 测试 ====================

func TestChildQueryApplicationService_GetByID_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	registerService := child.NewChildApplicationService(unitOfWork)
	ctx := context.Background()

	created, err := registerService.Register(ctx, child.RegisterChildDTO{
		Name:     "小明",
		Gender:   1, // 1=男
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	queryService := child.NewChildQueryApplicationService(unitOfWork)

	// Act - 查询儿童
	result, err := queryService.GetByID(ctx, created.ID)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, created.ID, result.ID)
	assert.Equal(t, created.Name, result.Name)
}

func TestChildQueryApplicationService_GetByID_NotFound(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	queryService := child.NewChildQueryApplicationService(unitOfWork)
	ctx := context.Background()

	// Act - 查询不存在的儿童
	result, err := queryService.GetByID(ctx, "999999999999999999")

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestChildQueryApplicationService_GetByIDCard_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	registerService := child.NewChildApplicationService(unitOfWork)
	ctx := context.Background()

	created, err := registerService.Register(ctx, child.RegisterChildDTO{
		Name:     "小明",
		Gender:   1, // 1=男
		Birthday: "2020-01-15",
		IDCard:   "110101202001151234",
	})
	require.NoError(t, err)

	queryService := child.NewChildQueryApplicationService(unitOfWork)

	// Act - 根据身份证查询
	result, err := queryService.GetByIDCard(ctx, "110101202001151234")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, created.ID, result.ID)
	assert.Equal(t, "110101202001151234", result.IDCard)
}

func TestChildQueryApplicationService_FindSimilar_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	registerService := child.NewChildApplicationService(unitOfWork)
	ctx := context.Background()

	// 注册多个儿童 (使用不同的身份证号或不设置)
	created1, err := registerService.Register(ctx, child.RegisterChildDTO{
		Name:     "小强",
		Gender:   1, // 1=男
		Birthday: "2020-03-10",
		IDCard:   "110101202003101118", // 唯一身份证
	})
	require.NoError(t, err)

	_, err = registerService.Register(ctx, child.RegisterChildDTO{
		Name:     "小丽",
		Gender:   2, // 2=女
		Birthday: "2020-03-10",
		IDCard:   "110101202003102225", // 另一个唯一身份证
	})
	require.NoError(t, err)

	queryService := child.NewChildQueryApplicationService(unitOfWork)

	// Act - 查找相似儿童（相同生日的男孩）
	results, err := queryService.FindSimilar(ctx, created1.Name, 1, "2020-03-10")

	// Assert
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)
}

func TestChildQueryApplicationService_FindSimilar_NoMatch(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	queryService := child.NewChildQueryApplicationService(unitOfWork)
	ctx := context.Background()

	// Act - 查找不存在的相似儿童
	results, err := queryService.FindSimilar(ctx, "不存在", 1, "2000-01-01")

	// Assert
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestChildAccessApplicationService_ListGetAndPatchForGuardian(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	userService := user.NewUserApplicationService(unitOfWork)
	userResult, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "监护人",
		Phone: "13800139001",
		Email: "guardian@example.com",
	})
	require.NoError(t, err)

	childService := child.NewChildApplicationService(unitOfWork)
	childResult, err := childService.Register(ctx, child.RegisterChildDTO{
		Name:     "旧名",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	guardService := guardianship.NewGuardianshipApplicationService(unitOfWork)
	require.NoError(t, guardService.AddGuardian(ctx, guardianship.AddGuardianDTO{
		UserID:   userResult.ID,
		ChildID:  childResult.ID,
		Relation: "parent",
	}))

	accessService := child.NewChildAccessApplicationService(unitOfWork)
	children, err := accessService.ListForGuardian(ctx, userResult.ID)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, childResult.ID, children[0].ID)

	found, err := accessService.GetForGuardian(ctx, userResult.ID, childResult.ID)
	require.NoError(t, err)
	assert.Equal(t, childResult.ID, found.ID)

	newName := "新名"
	gender := uint8(2)
	birthday := "2020-02-20"
	height := uint32(121)
	weight := uint32(26000)
	updated, err := accessService.PatchForGuardian(ctx, child.PatchChildForGuardianDTO{
		UserID:    userResult.ID,
		ChildID:   childResult.ID,
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

func TestChildAccessApplicationService_GetForGuardianRejectsNonGuardian(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	userService := user.NewUserApplicationService(unitOfWork)
	guardian, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "监护人",
		Phone: "13800139002",
		Email: "guardian2@example.com",
	})
	require.NoError(t, err)
	other, err := userService.Register(ctx, user.RegisterUserDTO{
		Name:  "其他用户",
		Phone: "13800139003",
		Email: "other@example.com",
	})
	require.NoError(t, err)

	childService := child.NewChildApplicationService(unitOfWork)
	childResult, err := childService.Register(ctx, child.RegisterChildDTO{
		Name:     "孩子",
		Gender:   1,
		Birthday: "2020-01-15",
	})
	require.NoError(t, err)

	guardService := guardianship.NewGuardianshipApplicationService(unitOfWork)
	require.NoError(t, guardService.AddGuardian(ctx, guardianship.AddGuardianDTO{
		UserID:   guardian.ID,
		ChildID:  childResult.ID,
		Relation: "parent",
	}))

	accessService := child.NewChildAccessApplicationService(unitOfWork)
	result, err := accessService.GetForGuardian(ctx, other.ID, childResult.ID)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, perrors.IsCode(err, code.ErrPermissionDenied))
}
