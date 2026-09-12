package assignment

import (
	"strings"

	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v5/internal/pkg/code"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
)

// Assignment 角色分配实体，表达主体直接持有某个角色的事实。
type Assignment struct {
	ID AssignmentID // 角色分配 ID

	// ---- 分配主体 ----
	SubjectType SubjectType // 主体类型：user/group/service
	SubjectID   meta.ID     // 对应主体类型下的主体 ID

	// ---- 分配角色 ----
	RoleID meta.ID // 角色ID

	// ---- 数据范围 ----
	// 未配置范围不表示全公司；业务数据访问需要显式范围。
	dataScope *scope.Scope

	// ---- 分配来源 ----
	GrantedBy string // 分配者标识
}

// NewAssignment 创建角色分配实体。
func NewAssignment(subjectType SubjectType, subjectID meta.ID, roleID meta.ID, opts ...Option) (Assignment, error) {
	subjectType = SubjectType(strings.TrimSpace(string(subjectType)))
	a := Assignment{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		RoleID:      roleID,
	}
	for _, opt := range opts {
		opt(&a)
	}
	if a.dataScope != nil && a.dataScope.IsZero() {
		return Assignment{}, perrors.WithCode(code.ErrInvalidArgument, "assignment scope must be explicit and valid")
	}
	a.GrantedBy = strings.TrimSpace(a.GrantedBy)
	if _, err := subject.NewRef(subject.Type(subjectType), subjectID); err != nil {
		return Assignment{}, err
	}
	if roleID.IsZero() {
		return Assignment{}, perrors.WithCode(code.ErrInvalidArgument, "role id is required")
	}
	if a.GrantedBy == "" {
		return Assignment{}, perrors.WithCode(code.ErrInvalidArgument, "granted by is required")
	}
	return a, nil
}

// Option 配置赋权事实。
type Option func(*Assignment)

func WithID(id AssignmentID) Option  { return func(a *Assignment) { a.ID = id } }
func WithGrantedBy(by string) Option { return func(a *Assignment) { a.GrantedBy = by } }

// WithScope 绑定本次角色分配的数据范围，不修改角色的动作授权。
func WithScope(value scope.Scope) Option {
	return func(a *Assignment) { a.dataScope = &value }
}

// Scope 返回范围值。未配置时返回 false，不能推断为全公司范围。
func (a Assignment) Scope() (scope.Scope, bool) {
	if a.dataScope == nil {
		return scope.Scope{}, false
	}
	return *a.dataScope, true
}

// AssignmentID 是赋权事实的标识。
type AssignmentID meta.ID

func NewAssignmentID(value uint64) AssignmentID {
	id := meta.FromUint64(value) // 来自 URL 或内部生成
	return AssignmentID(id)
}

func (id AssignmentID) Uint64() uint64 {
	return meta.ID(id).Uint64()
}

func (id AssignmentID) String() string {
	return meta.ID(id).String()
}

// SubjectType 主体类型
type SubjectType = subject.Type

const (
	SubjectTypeUser    = subject.TypeUser
	SubjectTypeGroup   = subject.TypeGroup
	SubjectTypeService = subject.TypeService
)

func (a Assignment) SubjectTypeString() string {
	return string(a.SubjectType)
}

// Fact 使用角色稳定名称表达主体持有角色的事实投影，不是另一份持久化角色分配实体。
type Fact struct {
	Subject  subject.Ref
	RoleName role.Name

	GrantedBy string
}

func NewFact(sub subject.Ref, roleName, grantedBy string) (Fact, error) {
	grantedBy = strings.TrimSpace(grantedBy)
	if sub.IsZero() {
		return Fact{}, perrors.WithCode(code.ErrInvalidArgument, "subject is required")
	}
	roleNameValue, err := role.NewName(roleName)
	if err != nil {
		return Fact{}, err
	}
	return Fact{Subject: sub, RoleName: roleNameValue, GrantedBy: grantedBy}, nil
}

func (f Fact) RoleNameString() string {
	return f.RoleName.String()
}
