package resource

import (
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// Action identifies an operation in an authorization request or policy fact.
type Action string

func NewAction(value string) (Action, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", perrors.WithCode(code.ErrInvalidArgument, "action is required")
	}
	if strings.Contains(value, "|") || strings.Contains(value, "*") {
		return "", perrors.WithCode(code.ErrInvalidArgument, "action must be a concrete operation")
	}
	return Action(value), nil
}

func (a Action) String() string {
	return string(a)
}

// ActionPattern identifies an operation expression stored in authorization facts.
type ActionPattern string

func NewActionPattern(value string) (ActionPattern, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", perrors.WithCode(code.ErrInvalidArgument, "action pattern is required")
	}
	return ActionPattern(value), nil
}

func (p ActionPattern) String() string {
	return string(p)
}

// CatalogAction describes a well-known action shape used by catalogs.
type CatalogAction struct {
	Name        string // 动作名称，如 read_all, read_own
	DisplayName string // 显示名称
	Scope       Scope  // all/own，用于区分是否需要 owner 校验
}

// Scope 动作作用域
type Scope string

const (
	ScopeAll Scope = "all" // 全局作用域
	ScopeOwn Scope = "own" // 仅自己
)

// 预定义动作枚举（V1 最小集）
var (
	ActionCreate     = CatalogAction{Name: "create", DisplayName: "创建", Scope: ScopeOwn}
	ActionReadAll    = CatalogAction{Name: "read_all", DisplayName: "读取(全部)", Scope: ScopeAll}
	ActionReadOwn    = CatalogAction{Name: "read_own", DisplayName: "读取(自己)", Scope: ScopeOwn}
	ActionUpdateAll  = CatalogAction{Name: "update_all", DisplayName: "更新(全部)", Scope: ScopeAll}
	ActionUpdateOwn  = CatalogAction{Name: "update_own", DisplayName: "更新(自己)", Scope: ScopeOwn}
	ActionDeleteAll  = CatalogAction{Name: "delete_all", DisplayName: "删除(全部)", Scope: ScopeAll}
	ActionDeleteOwn  = CatalogAction{Name: "delete_own", DisplayName: "删除(自己)", Scope: ScopeOwn}
	ActionApprove    = CatalogAction{Name: "approve", DisplayName: "审批", Scope: ScopeAll}
	ActionExport     = CatalogAction{Name: "export", DisplayName: "导出", Scope: ScopeAll}
	ActionDisableAll = CatalogAction{Name: "disable_all", DisplayName: "禁用(全部)", Scope: ScopeAll}
)

// StandardActions 标准动作集合
var StandardActions = []CatalogAction{
	ActionCreate,
	ActionReadAll,
	ActionReadOwn,
	ActionUpdateAll,
	ActionUpdateOwn,
	ActionDeleteAll,
	ActionDeleteOwn,
	ActionApprove,
	ActionExport,
	ActionDisableAll,
}

// GetActionByName 根据名称获取动作
func GetActionByName(name string) *CatalogAction {
	for _, action := range StandardActions {
		if action.Name == name {
			return &action
		}
	}
	return nil
}
