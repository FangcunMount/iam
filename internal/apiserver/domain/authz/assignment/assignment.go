package assignment

import (
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// Assignment 用户/组 ↔ 角色赋权（聚合根）
type Assignment struct {
	ID          AssignmentID
	SubjectType SubjectType // user/group/service
	SubjectID   string      // 用户或组ID
	RoleID      uint64      // 角色ID
	TenantID    string      // 租户ID（域）
	GrantedBy   string      // 授权人
}

// NewAssignment 创建新赋权
func NewAssignment(subjectType SubjectType, subjectID string, roleID uint64, tenantID string, opts ...AssignmentOption) Assignment {
	a := Assignment{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		RoleID:      roleID,
		TenantID:    tenantID,
	}
	for _, opt := range opts {
		opt(&a)
	}
	return a
}

// AssignmentOption 赋权选项
type AssignmentOption func(*Assignment)

func WithID(id AssignmentID) AssignmentOption  { return func(a *Assignment) { a.ID = id } }
func WithGrantedBy(by string) AssignmentOption { return func(a *Assignment) { a.GrantedBy = by } }

// SubjectKey 返回 Casbin 中的主体标识
func (a *Assignment) SubjectKey() string {
	return string(a.SubjectType) + ":" + a.SubjectID
}

// RoleKey 返回 Casbin 中的角色标识
func (a *Assignment) RoleKey() string {
	id := meta.FromUint64(a.RoleID) // RoleID 来自内部，必定有效
	return "role:" + id.String()
}

// AssignmentID 赋权ID值对象
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
type SubjectType string

const (
	SubjectTypeUser    SubjectType = "user"
	SubjectTypeGroup   SubjectType = "group"
	SubjectTypeService SubjectType = "service"
)

func (st SubjectType) String() string {
	return string(st)
}
