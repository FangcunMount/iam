package user

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	sessiondomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"gorm.io/gorm"
)

// ============= 应用服务实现 =============

// ======================================
// ==== UserApplicationService 实现 =====
// ======================================

// userApplicationService 用户应用服务实现
type userApplicationService struct {
	uow uow.UnitOfWork
}

// NewUserApplicationService 创建用户应用服务
func NewUserApplicationService(uow uow.UnitOfWork) UserApplicationService {
	return &userApplicationService{uow: uow}
}

// Register 注册新用户
func (s *userApplicationService) Register(ctx context.Context, dto RegisterUserDTO) (*UserResult, error) {
	l := logger.L(ctx)
	var result *UserResult

	l.Debugw("开始注册用户",
		"action", logger.ActionRegister,
		"resource", logger.ResourceUser,
		"name", dto.Name,
		"phone", dto.Phone,
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建验证器
		validator := user.NewValidator(tx.Users)

		// 转换 DTO 为值对象
		var phone meta.Phone
		if dto.Phone != "" {
			var err error
			phone, err = meta.NewPhone(dto.Phone)
			if err != nil {
				return err
			}
		}

		// 验证注册参数
		if err := validator.ValidateRegister(ctx, dto.Name, phone); err != nil {
			l.Warnw("用户注册参数验证失败",
				"action", logger.ActionRegister,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 创建用户实体（如有指定ID则使用）
		var opts []user.UserOption
		if dto.ID > 0 {
			opts = append(opts, user.WithID(meta.FromUint64(dto.ID)))
		}
		newUser, err := user.NewUser(dto.Name, phone, opts...)
		if err != nil {
			l.Errorw("创建用户实体失败",
				"action", logger.ActionRegister,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 设置可选的邮箱
		if dto.Email != "" {
			email, err := meta.NewEmail(dto.Email)
			if err != nil {
				l.Warnw("邮箱格式验证失败",
					"action", logger.ActionRegister,
					"resource", logger.ResourceUser,
					"error", err.Error(),
				)
				return err
			}
			newUser.UpdateEmail(email)
		}

		// 持久化用户
		if err := tx.Users.Create(ctx, newUser); err != nil {
			l.Errorw("持久化用户失败",
				"action", logger.ActionRegister,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 转换为 DTO
		result = toUserResult(newUser)
		return nil
	})

	if err == nil {
		l.Debugw("用户注册成功",
			"action", logger.ActionRegister,
			"resource", logger.ResourceUser,
			"user_id", result.ID,
			"result", logger.ResultSuccess,
		)
	}

	return result, err
}

// =============================================
// ==== UserProfileApplicationService 实现 =====
// =============================================

// userProfileApplicationService 用户资料应用服务实现
type userProfileApplicationService struct {
	uow uow.UnitOfWork
}

// NewUserProfileApplicationService 创建用户资料应用服务
func NewUserProfileApplicationService(uow uow.UnitOfWork) UserProfileApplicationService {
	return &userProfileApplicationService{uow: uow}
}

// Rename 修改用户名称
func (s *userProfileApplicationService) Rename(ctx context.Context, userID string, newName string) error {
	l := logger.L(ctx)
	l.Debugw("修改用户名称",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", userID,
		"new_name", newName,
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建领域服务
		validator := user.NewValidator(tx.Users)
		profileEditor := user.NewProfileEditor(tx.Users, validator)

		// 转换 ID
		id, err := parseUserID(userID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			return err
		}

		// 调用领域服务修改名称
		modifiedUser, err := profileEditor.Rename(ctx, id, newName)
		if err != nil {
			l.Errorw("修改用户名称失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Users.Update(ctx, modifiedUser)
	})

	if err == nil {
		l.Debugw("修改用户名称成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceUser,
			"user_id", userID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// Renickname 修改用户昵称
func (s *userProfileApplicationService) Renickname(ctx context.Context, userID string, newNickname string) error {
	l := logger.L(ctx)
	l.Debugw("修改用户昵称",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", userID,
		"new_nickname", newNickname,
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建领域服务
		validator := user.NewValidator(tx.Users)
		profileEditor := user.NewProfileEditor(tx.Users, validator)

		// 转换 ID
		id, err := parseUserID(userID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			return err
		}

		// 调用领域服务修改昵称
		modifiedUser, err := profileEditor.Renickname(ctx, id, newNickname)
		if err != nil {
			l.Errorw("修改用户昵称失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Users.Update(ctx, modifiedUser)
	})

	if err == nil {
		l.Debugw("修改用户昵称成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceUser,
			"user_id", userID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// UpdateContact 更新联系方式
func (s *userProfileApplicationService) UpdateContact(ctx context.Context, dto UpdateContactDTO) error {
	l := logger.L(ctx)
	l.Debugw("更新用户联系方式",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", dto.UserID,
		"phone", dto.Phone,
		"email", dto.Email,
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建领域服务
		validator := user.NewValidator(tx.Users)
		profileEditor := user.NewProfileEditor(tx.Users, validator)

		// 转换 ID
		id, err := parseUserID(dto.UserID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			return err
		}

		// 转换值对象
		var phone meta.Phone
		if dto.Phone != "" {
			phone, err = meta.NewPhone(dto.Phone)
			if err != nil {
				l.Warnw("手机号格式错误",
					"action", logger.ActionUpdate,
					"resource", logger.ResourceUser,
					"error", err.Error(),
				)
				return err
			}
		}
		var email meta.Email
		if dto.Email != "" {
			email, err = meta.NewEmail(dto.Email)
			if err != nil {
				l.Warnw("邮箱格式错误",
					"action", logger.ActionUpdate,
					"resource", logger.ResourceUser,
					"error", err.Error(),
				)
				return err
			}
		}

		// 调用领域服务更新联系方式
		modifiedUser, err := profileEditor.UpdateContact(ctx, id, phone, email)
		if err != nil {
			l.Errorw("更新联系方式失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Users.Update(ctx, modifiedUser)
	})

	if err == nil {
		l.Debugw("更新联系方式成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceUser,
			"user_id", dto.UserID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// UpdateIDCard 更新身份证
func (s *userProfileApplicationService) UpdateIDCard(ctx context.Context, userID string, idCard string) error {
	l := logger.L(ctx)
	l.Debugw("更新用户身份证",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", userID,
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建领域服务
		validator := user.NewValidator(tx.Users)
		profileEditor := user.NewProfileEditor(tx.Users, validator)

		// 转换 ID
		id, err := parseUserID(userID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			return err
		}

		// 转换身份证 (NewIDCard 需要name和number两个参数，这里我们只传number，name留空)
		idCardVO, err := meta.NewIDCard("", idCard)
		if err != nil {
			l.Warnw("身份证格式错误",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			return err
		}

		// 调用领域服务更新身份证
		modifiedUser, err := profileEditor.UpdateIDCard(ctx, id, idCardVO)
		if err != nil {
			l.Errorw("更新身份证失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Users.Update(ctx, modifiedUser)
	})

	if err == nil {
		l.Debugw("更新身份证成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceUser,
			"user_id", userID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// ===========================================
// ==== UserStatusApplicationService 实现 =====
// ===========================================

// userStatusApplicationService 用户状态应用服务实现
type userStatusApplicationService struct {
	uow            uow.UnitOfWork
	sessionManager sessiondomain.Manager
}

// NewUserStatusApplicationService 创建用户状态应用服务
func NewUserStatusApplicationService(uow uow.UnitOfWork, sessionManager sessiondomain.Manager) UserStatusApplicationService {
	return &userStatusApplicationService{uow: uow, sessionManager: sessionManager}
}

// Activate 激活用户
func (s *userStatusApplicationService) Activate(ctx context.Context, userID string) error {
	l := logger.L(ctx)
	l.Debugw("激活用户",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", userID,
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建领域服务
		lifecycler := user.NewLifecycler(tx.Users)

		// 转换 ID
		id, err := parseUserID(userID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			return err
		}

		// 调用领域服务激活用户
		modifiedUser, err := lifecycler.Activate(ctx, id)
		if err != nil {
			l.Errorw("激活用户失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Users.Update(ctx, modifiedUser)
	})

	if err == nil {
		l.Debugw("激活用户成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceUser,
			"user_id", userID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// Deactivate 停用用户
func (s *userStatusApplicationService) Deactivate(ctx context.Context, userID string) error {
	l := logger.L(ctx)
	l.Debugw("停用用户",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", userID,
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建领域服务
		lifecycler := user.NewLifecycler(tx.Users)

		// 转换 ID
		id, err := parseUserID(userID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			return err
		}

		// 调用领域服务停用用户
		modifiedUser, err := lifecycler.Deactivate(ctx, id)
		if err != nil {
			l.Errorw("停用用户失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Users.Update(ctx, modifiedUser)
	})

	if err == nil {
		l.Debugw("停用用户成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceUser,
			"user_id", userID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// Block 封禁用户
func (s *userStatusApplicationService) Block(ctx context.Context, userID string) error {
	l := logger.L(ctx)
	l.Debugw("封禁用户",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", userID,
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建领域服务
		lifecycler := user.NewLifecycler(tx.Users)

		// 转换 ID
		id, err := parseUserID(userID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			return err
		}

		// 调用领域服务封禁用户
		modifiedUser, err := lifecycler.Block(ctx, id)
		if err != nil {
			l.Errorw("封禁用户失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Users.Update(ctx, modifiedUser)
	})

	if err == nil {
		l.Debugw("封禁用户成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceUser,
			"user_id", userID,
			"result", logger.ResultSuccess,
		)
	}

	if err != nil {
		return err
	}
	if s.sessionManager == nil {
		return nil
	}
	id, parseErr := parseUserID(userID)
	if parseErr != nil {
		return parseErr
	}
	return s.sessionManager.RevokeByUser(ctx, id, "user_blocked", userID)
}

// ===========================================
// ==== UserQueryApplicationService 实现 =====
// ===========================================

// userQueryApplicationService 用户查询应用服务实现
type userQueryApplicationService struct {
	uow uow.UnitOfWork
}

// NewUserQueryApplicationService 创建用户查询应用服务
func NewUserQueryApplicationService(uow uow.UnitOfWork) UserQueryApplicationService {
	return &userQueryApplicationService{uow: uow}
}

// GetByID 根据 ID 查询用户
func (s *userQueryApplicationService) GetByID(ctx context.Context, userID string) (*UserResult, error) {
	l := logger.L(ctx)
	l.Debugw("查询用户信息",
		"action", logger.ActionRead,
		"resource", logger.ResourceUser,
		"user_id", userID,
	)

	var result *UserResult

	// 查询操作也通过 UoW，但不需要写操作，可以直接使用只读事务
	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {

		userIDObj, err := parseUserID(userID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionRead,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			return err
		}

		user, err := tx.Users.FindByID(ctx, userIDObj)
		if err != nil {
			l.Warnw("用户不存在",
				"action", logger.ActionRead,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			if isUserNotFound(err) {
				return perrors.WithCode(code.ErrUserNotFound, "user(%s) not found", userID)
			}
			return err
		}

		result = toUserResult(user)
		return nil
	})

	if err == nil {
		l.Debugw("查询用户成功",
			"action", logger.ActionRead,
			"resource", logger.ResourceUser,
			"user_id", result.ID,
			"result", logger.ResultSuccess,
		)
	}

	return result, err
}

// GetByPhone 根据手机号查询用户
func (s *userQueryApplicationService) GetByPhone(ctx context.Context, phone string) (*UserResult, error) {
	var result *UserResult

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {

		phoneObj, err := meta.NewPhone(phone)
		if err != nil {
			return err
		}

		user, err := tx.Users.FindByPhone(ctx, phoneObj)
		if err != nil {
			return err
		}

		result = toUserResult(user)
		return nil
	})

	return result, err
}

// ============= DTO 转换辅助函数 =============

// parseUserID 解析用户ID字符串
func parseUserID(userID string) (meta.ID, error) {
	// 将字符串转换为uint64
	var id uint64
	_, err := fmt.Sscanf(userID, "%d", &id)
	if err != nil {
		return meta.FromUint64(0), err
	}
	return meta.FromUint64(id), nil
}

// toUserResult 将领域实体转换为 DTO
func toUserResult(user *domain.User) *UserResult {
	if user == nil {
		return nil
	}

	return &UserResult{
		ID:     user.ID.String(),
		Name:   user.Name,
		Phone:  user.Phone.String(),
		Email:  user.Email.String(),
		IDCard: user.IDCard.String(),
		Status: user.Status,
	}
}

func isUserNotFound(err error) bool {
	return perrors.Is(err, gorm.ErrRecordNotFound) || perrors.IsCode(err, code.ErrUserNotFound)
}
