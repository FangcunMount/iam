package user_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/internal/apiserver/application/uc/testutil"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/user"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
)

// ==================== UserApplicationService 测试 ====================

func TestUserApplicationService_Register_Success(t *testing.T) {
	// Arrange - 准备测试环境
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	appService := user.NewUserApplicationService(unitOfWork)
	ctx := context.Background()

	dto := user.RegisterUserDTO{
		Name:  "张三",
		Phone: "13800138000",
		Email: "zhangsan@example.com",
	}

	// Act - 执行注册
	result, err := appService.Register(ctx, dto)

	// Assert - 验证结果
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, dto.Name, result.Name)
	assert.Equal(t, "+86"+dto.Phone, result.Phone) // Phone 会被规范化为 E.164 格式
	assert.Equal(t, dto.Email, result.Email)
	assert.Equal(t, domain.UserActive, result.Status)

	// 验证数据库持久化
	queryService := user.NewUserQueryApplicationService(unitOfWork)
	saved, err := queryService.GetByPhone(ctx, "+86"+dto.Phone) // 查询时也需要使用 E.164 格式
	require.NoError(t, err)
	assert.Equal(t, result.ID, saved.ID)
}

func TestUserApplicationService_Register_WithoutEmail(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	appService := user.NewUserApplicationService(unitOfWork)
	ctx := context.Background()

	dto := user.RegisterUserDTO{
		Name:  "李四",
		Phone: "13800138001",
		Email: "", // 不提供邮箱
	}

	// Act
	result, err := appService.Register(ctx, dto)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, dto.Name, result.Name)
	assert.Equal(t, "+86"+dto.Phone, result.Phone) // Phone 会被规范化为 E.164 格式
	assert.Empty(t, result.Email)                  // 邮箱应该为空
}

func TestUserApplicationService_Register_DuplicatePhone(t *testing.T) {
	// Arrange - 先注册一个用户
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	appService := user.NewUserApplicationService(unitOfWork)
	ctx := context.Background()

	dto1 := user.RegisterUserDTO{
		Name:  "张三",
		Phone: "13800138000",
	}
	_, err := appService.Register(ctx, dto1)
	require.NoError(t, err)

	// Act - 尝试注册相同手机号
	dto2 := user.RegisterUserDTO{
		Name:  "李四",
		Phone: "13800138000", // 重复手机号
	}
	result, err := appService.Register(ctx, dto2)

	// Assert - 应该失败
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestUserApplicationService_Register_InvalidPhone(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	appService := user.NewUserApplicationService(unitOfWork)
	ctx := context.Background()

	dto := user.RegisterUserDTO{
		Name:  "王五",
		Phone: "not-a-phone", // 非法电话号码
	}

	// Act
	result, err := appService.Register(ctx, dto)

	// Assert - 非法电话号码应该被拒绝
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestUserApplicationService_Register_WithoutPhone(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	appService := user.NewUserApplicationService(unitOfWork)
	ctx := context.Background()

	dto := user.RegisterUserDTO{
		Name:  "赵六",
		Phone: "",
		Email: "zhaoliu@example.com",
	}

	// Act
	result, err := appService.Register(ctx, dto)

	// Assert - 空手机号允许
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, dto.Name, result.Name)
	assert.Empty(t, result.Phone)
	assert.Equal(t, dto.Email, result.Email)
}

// ==================== UserProfileApplicationService 测试 ====================

func TestUserProfileApplicationService_Rename_Success(t *testing.T) {
	// Arrange - 先创建一个用户
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	registerService := user.NewUserApplicationService(unitOfWork)
	ctx := context.Background()

	created, err := registerService.Register(ctx, user.RegisterUserDTO{
		Name:  "张三",
		Phone: "13800138000",
	})
	require.NoError(t, err)

	profileService := user.NewUserProfileApplicationService(unitOfWork)
	newName := "张三丰"

	// Act - 修改名称
	err = profileService.Rename(ctx, created.ID, newName)

	// Assert
	require.NoError(t, err)

	// 验证修改结果
	queryService := user.NewUserQueryApplicationService(unitOfWork)
	updated, err := queryService.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, newName, updated.Name)
}

func TestUserProfileApplicationService_Rename_EmptyName(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	registerService := user.NewUserApplicationService(unitOfWork)
	ctx := context.Background()

	created, err := registerService.Register(ctx, user.RegisterUserDTO{
		Name:  "张三",
		Phone: "13800138000",
	})
	require.NoError(t, err)

	profileService := user.NewUserProfileApplicationService(unitOfWork)

	// Act - 尝试设置空名称
	err = profileService.Rename(ctx, created.ID, "")

	// Assert - 空名称应该被拒绝
	require.Error(t, err)
}

func TestUserProfileApplicationService_UpdateContact_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	registerService := user.NewUserApplicationService(unitOfWork)
	ctx := context.Background()

	created, err := registerService.Register(ctx, user.RegisterUserDTO{
		Name:  "张三",
		Phone: "13800138000",
		Email: "old@example.com",
	})
	require.NoError(t, err)

	profileService := user.NewUserProfileApplicationService(unitOfWork)
	dto := user.UpdateContactDTO{
		UserID: created.ID,
		Phone:  "13900139000",     // 新手机号
		Email:  "new@example.com", // 新邮箱
	}

	// Act
	err = profileService.UpdateContact(ctx, dto)

	// Assert
	require.NoError(t, err)

	// 验证修改结果
	queryService := user.NewUserQueryApplicationService(unitOfWork)
	updated, err := queryService.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "+86"+dto.Phone, updated.Phone) // Phone 会被规范化为 E.164 格式
	assert.Equal(t, dto.Email, updated.Email)
}

func TestUserProfileApplicationService_PatchProfile_OrchestratesProfileAndContact(t *testing.T) {
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	registerService := user.NewUserApplicationService(unitOfWork)
	created, err := registerService.Register(ctx, user.RegisterUserDTO{
		Name:  "张三",
		Phone: "13800138000",
		Email: "old@example.com",
	})
	require.NoError(t, err)

	profileService := user.NewUserProfileApplicationService(unitOfWork)
	nickname := "张三丰"
	phone := "13900139000"
	email := "new@example.com"

	updated, err := profileService.PatchProfile(ctx, user.PatchUserProfileDTO{
		UserID:   created.ID,
		Nickname: &nickname,
		Phone:    &phone,
		Email:    &email,
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, nickname, updated.Name)
	assert.Equal(t, "+86"+phone, updated.Phone)
	assert.Equal(t, email, updated.Email)
}

func TestUserProfileApplicationService_UpdateIDCard_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)

	registerService := user.NewUserApplicationService(unitOfWork)
	ctx := context.Background()

	created, err := registerService.Register(ctx, user.RegisterUserDTO{
		Name:  "张三",
		Phone: "13800138000",
	})
	require.NoError(t, err)

	profileService := user.NewUserProfileApplicationService(unitOfWork)
	idCard := "110101199003070011" // 有效的测试身份证号

	// Act
	err = profileService.UpdateIDCard(ctx, created.ID, idCard)

	// Assert
	require.NoError(t, err)

	// 验证修改结果
	queryService := user.NewUserQueryApplicationService(unitOfWork)
	updated, err := queryService.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, idCard, updated.IDCard)
}

// ==================== UserStatusApplicationService 测试 ====================

func TestUserStatusApplicationService_Activate_Success(t *testing.T) {
	// Arrange - 先创建一个停用的用户
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	registerService := user.NewUserApplicationService(unitOfWork)
	created, err := registerService.Register(ctx, user.RegisterUserDTO{
		Name:  "张三",
		Phone: "13800138000",
	})
	require.NoError(t, err)

	// 先停用
	statusService := user.NewUserStatusApplicationService(unitOfWork, nil)
	err = statusService.Deactivate(ctx, created.ID)
	require.NoError(t, err)

	// Act - 激活
	err = statusService.Activate(ctx, created.ID)

	// Assert
	require.NoError(t, err)

	// 验证状态
	queryService := user.NewUserQueryApplicationService(unitOfWork)
	updated, err := queryService.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.UserActive, updated.Status)
}

func TestUserStatusApplicationService_Deactivate_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	registerService := user.NewUserApplicationService(unitOfWork)
	created, err := registerService.Register(ctx, user.RegisterUserDTO{
		Name:  "张三",
		Phone: "13800138000",
	})
	require.NoError(t, err)

	statusService := user.NewUserStatusApplicationService(unitOfWork, nil)

	// Act - 停用
	err = statusService.Deactivate(ctx, created.ID)

	// Assert
	require.NoError(t, err)

	// 验证状态
	queryService := user.NewUserQueryApplicationService(unitOfWork)
	updated, err := queryService.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.UserInactive, updated.Status)
}

func TestUserStatusApplicationService_Block_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	registerService := user.NewUserApplicationService(unitOfWork)
	created, err := registerService.Register(ctx, user.RegisterUserDTO{
		Name:  "张三",
		Phone: "13800138000",
	})
	require.NoError(t, err)

	statusService := user.NewUserStatusApplicationService(unitOfWork, nil)

	// Act - 封禁
	err = statusService.Block(ctx, created.ID)

	// Assert
	require.NoError(t, err)

	// 验证状态
	queryService := user.NewUserQueryApplicationService(unitOfWork)
	updated, err := queryService.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.UserBlocked, updated.Status)
}

// ==================== UserQueryApplicationService 测试 ====================

func TestUserQueryApplicationService_GetByID_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	registerService := user.NewUserApplicationService(unitOfWork)
	created, err := registerService.Register(ctx, user.RegisterUserDTO{
		Name:  "张三",
		Phone: "13800138000",
		Email: "zhangsan@example.com",
	})
	require.NoError(t, err)

	queryService := user.NewUserQueryApplicationService(unitOfWork)

	// Act
	result, err := queryService.GetByID(ctx, created.ID)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, created.ID, result.ID)
	assert.Equal(t, created.Name, result.Name)
	assert.Equal(t, created.Phone, result.Phone)
	assert.Equal(t, created.Email, result.Email)
}

func TestUserQueryApplicationService_GetByID_NotFound(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	queryService := user.NewUserQueryApplicationService(unitOfWork)
	ctx := context.Background()

	// Act - 查询不存在的用户
	result, err := queryService.GetByID(ctx, "99999")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestUserQueryApplicationService_GetByPhone_Success(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	registerService := user.NewUserApplicationService(unitOfWork)
	created, err := registerService.Register(ctx, user.RegisterUserDTO{
		Name:  "张三",
		Phone: "13800138000",
	})
	require.NoError(t, err)

	queryService := user.NewUserQueryApplicationService(unitOfWork)

	// Act
	result, err := queryService.GetByPhone(ctx, created.Phone)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, created.ID, result.ID)
	assert.Equal(t, created.Phone, result.Phone)
}

func TestUserQueryApplicationService_GetByPhone_NotFound(t *testing.T) {
	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	queryService := user.NewUserQueryApplicationService(unitOfWork)
	ctx := context.Background()

	// Act
	result, err := queryService.GetByPhone(ctx, "19999999999")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
}

// ==================== 事务测试 ====================

func TestUserApplicationService_Transaction_Rollback(t *testing.T) {
	// 此测试验证事务回滚功能
	// 由于我们使用的是内存数据库和真实的 UnitOfWork，
	// 如果中间出现错误，事务应该自动回滚

	// Arrange
	db := testutil.SetupTestDB(t)
	unitOfWork := testutil.NewUnitOfWork(db)
	ctx := context.Background()

	// 先注册一个用户
	registerService := user.NewUserApplicationService(unitOfWork)
	_, err := registerService.Register(ctx, user.RegisterUserDTO{
		Name:  "张三",
		Phone: "13800138000",
	})
	require.NoError(t, err)

	// Act - 尝试注册重复手机号（应该在事务中失败并回滚）
	_, err = registerService.Register(ctx, user.RegisterUserDTO{
		Name:  "李四",
		Phone: "13800138000", // 重复
	})

	// Assert - 注册应该失败
	require.Error(t, err)

	// 验证数据库中只有一个用户
	queryService := user.NewUserQueryApplicationService(unitOfWork)
	result, err := queryService.GetByPhone(ctx, "13800138000")
	require.NoError(t, err)
	assert.Equal(t, "张三", result.Name) // 应该是第一个用户
}
