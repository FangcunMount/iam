package account

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// statusManager 账号状态管理器实现
// 职责：管理账号状态转换，封装状态转换业务规则
type statusManager struct {
	repo Repository
}

// 确保实现了 StatusManager 接口
var _ StatusManager = (*statusManager)(nil)

// NewStatusManager 创建账号状态管理器实例
func NewStatusManager(repo Repository) StatusManager {
	return &statusManager{
		repo: repo,
	}
}

// Activate 激活账号
func (sm *statusManager) Activate(ctx context.Context, accountID meta.ID) (*Account, error) {
	account, err := sm.repo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, perrors.WithCode(code.ErrNotFoundAccount, "account not found")
	}

	// 使用状态机进行状态转换
	machine, err := newAccountStateMachine(account)
	if err != nil {
		return nil, err
	}

	if err := machine.activate(); err != nil {
		return nil, err
	}

	return account, nil
}

// Disable 禁用账号
func (sm *statusManager) Disable(ctx context.Context, accountID meta.ID) (*Account, error) {
	account, err := sm.repo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, perrors.WithCode(code.ErrNotFoundAccount, "account not found")
	}

	// 使用状态机进行状态转换
	machine, err := newAccountStateMachine(account)
	if err != nil {
		return nil, err
	}

	if err := machine.disable(); err != nil {
		return nil, err
	}

	return account, nil
}

// Archive 归档账号
func (sm *statusManager) Archive(ctx context.Context, accountID meta.ID) (*Account, error) {
	account, err := sm.repo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, perrors.WithCode(code.ErrNotFoundAccount, "account not found")
	}

	// 使用状态机进行状态转换
	machine, err := newAccountStateMachine(account)
	if err != nil {
		return nil, err
	}

	if err := machine.archive(); err != nil {
		return nil, err
	}

	return account, nil
}

// Delete 删除账号
func (sm *statusManager) Delete(ctx context.Context, accountID meta.ID) (*Account, error) {
	account, err := sm.repo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, perrors.WithCode(code.ErrNotFoundAccount, "account not found")
	}

	// 使用状态机进行状态转换
	machine, err := newAccountStateMachine(account)
	if err != nil {
		return nil, err
	}

	if err := machine.delete(); err != nil {
		return nil, err
	}

	return account, nil
}

// ==================== 内部状态机实现 ====================

// accountStateMachine 账号状态机（内部使用）
type accountStateMachine struct {
	account *Account
	state   accountState
}

// newAccountStateMachine 创建账号状态机（内部使用）
func newAccountStateMachine(account *Account) (*accountStateMachine, error) {
	if account == nil {
		return nil, perrors.WithCode(code.ErrInvalidStateTransition, "account cannot be nil")
	}

	m := &accountStateMachine{
		account: account,
	}

	if err := m.setState(account.Status); err != nil {
		return nil, err
	}

	return m, nil
}

// setState 设置状态机的状态
func (m *accountStateMachine) setState(status AccountStatus) error {
	switch status {
	case StatusDisabled:
		m.state = &disabledAccountState{}
	case StatusActive:
		m.state = &activeAccountState{}
	case StatusArchived:
		m.state = &archivedAccountState{}
	case StatusDeleted:
		m.state = &deletedAccountState{}
	default:
		return perrors.WithCode(code.ErrInvalidStateTransition, "invalid account status: %d", status)
	}
	return nil
}

func (m *accountStateMachine) activate() error {
	return m.state.activate(m)
}

func (m *accountStateMachine) disable() error {
	return m.state.disable(m)
}

func (m *accountStateMachine) archive() error {
	return m.state.archive(m)
}

func (m *accountStateMachine) delete() error {
	return m.state.delete(m)
}

// transitionTo 执行状态转换
func (m *accountStateMachine) transitionTo(newStatus AccountStatus) error {
	oldStatus := m.account.Status

	if oldStatus == newStatus {
		log.Infow("Account status transition (idempotent)",
			"accountID", m.account.ID,
			"from", oldStatus.String(),
			"to", newStatus.String(),
		)
		return nil
	}

	if !m.account.CanTransitionTo(newStatus) {
		return m.invalidTransition(newStatus)
	}

	switch newStatus {
	case StatusActive:
		m.account.Activate()
	case StatusDisabled:
		m.account.Disable()
	case StatusArchived:
		m.account.Archive()
	case StatusDeleted:
		m.account.Delete()
	default:
		return perrors.WithCode(code.ErrInvalidStateTransition, "unknown target status: %d", newStatus)
	}

	if err := m.setState(newStatus); err != nil {
		m.account.Status = oldStatus
		_ = m.setState(oldStatus)
		return err
	}

	log.Infow("Account status transition succeeded",
		"accountID", m.account.ID,
		"from", oldStatus.String(),
		"to", newStatus.String(),
	)

	return nil
}

func (m *accountStateMachine) invalidTransition(targetStatus AccountStatus) error {
	log.Warnw("Invalid account status transition",
		"accountID", m.account.ID,
		"from", m.account.Status.String(),
		"to", targetStatus.String(),
	)
	return perrors.WithCode(
		code.ErrInvalidStateTransition,
		"cannot transition from %s to %s",
		m.account.Status.String(),
		targetStatus.String(),
	)
}

func (m *accountStateMachine) String() string {
	return fmt.Sprintf("accountStateMachine{accountID=%s, status=%s}", m.account.ID, m.account.Status.String())
}

// ==================== 状态接口和实现 ====================

type accountState interface {
	activate(m *accountStateMachine) error
	disable(m *accountStateMachine) error
	archive(m *accountStateMachine) error
	delete(m *accountStateMachine) error
}

type disabledAccountState struct{}

func (s *disabledAccountState) activate(m *accountStateMachine) error {
	return m.transitionTo(StatusActive)
}

func (s *disabledAccountState) disable(m *accountStateMachine) error {
	return m.transitionTo(StatusDisabled)
}

func (s *disabledAccountState) archive(m *accountStateMachine) error {
	return m.transitionTo(StatusArchived)
}

func (s *disabledAccountState) delete(m *accountStateMachine) error {
	return m.transitionTo(StatusDeleted)
}

type activeAccountState struct{}

func (s *activeAccountState) activate(m *accountStateMachine) error {
	return m.transitionTo(StatusActive)
}

func (s *activeAccountState) disable(m *accountStateMachine) error {
	return m.transitionTo(StatusDisabled)
}

func (s *activeAccountState) archive(m *accountStateMachine) error {
	return m.transitionTo(StatusArchived)
}

func (s *activeAccountState) delete(m *accountStateMachine) error {
	return m.transitionTo(StatusDeleted)
}

type archivedAccountState struct{}

func (s *archivedAccountState) activate(m *accountStateMachine) error {
	return m.transitionTo(StatusActive)
}

func (s *archivedAccountState) disable(m *accountStateMachine) error {
	return m.invalidTransition(StatusDisabled)
}

func (s *archivedAccountState) archive(m *accountStateMachine) error {
	return m.transitionTo(StatusArchived)
}

func (s *archivedAccountState) delete(m *accountStateMachine) error {
	return m.transitionTo(StatusDeleted)
}

type deletedAccountState struct{}

func (s *deletedAccountState) activate(m *accountStateMachine) error {
	return m.invalidTransition(StatusActive)
}

func (s *deletedAccountState) disable(m *accountStateMachine) error {
	return m.invalidTransition(StatusDisabled)
}

func (s *deletedAccountState) archive(m *accountStateMachine) error {
	return m.invalidTransition(StatusArchived)
}

func (s *deletedAccountState) delete(m *accountStateMachine) error {
	return m.transitionTo(StatusDeleted)
}
