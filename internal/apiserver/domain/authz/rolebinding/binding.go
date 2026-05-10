package rolebinding

import (
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// Binding 用户/组 ↔ 角色赋权（聚合根）
type Binding struct {
	ID          BindingID
	SubjectType SubjectType // user/group/service
	SubjectID   meta.ID     // 用户或组ID
	RoleID      meta.ID     // 角色ID
	TenantID    string      // 租户ID（域）
	GrantedBy   string      // 授权人
}

// NewBinding 创建新赋权
func NewBinding(subjectType SubjectType, subjectID meta.ID, roleID meta.ID, tenantID string, opts ...BindingOption) Binding {
	a := Binding{
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

// BindingOption 赋权选项
type BindingOption func(*Binding)

func WithID(id BindingID) BindingOption     { return func(a *Binding) { a.ID = id } }
func WithGrantedBy(by string) BindingOption { return func(a *Binding) { a.GrantedBy = by } }

// BindingID 赋权ID值对象
type BindingID meta.ID

func NewBindingID(value uint64) BindingID {
	id := meta.FromUint64(value) // 来自 URL 或内部生成
	return BindingID(id)
}

func (id BindingID) Uint64() uint64 {
	return meta.ID(id).Uint64()
}

func (id BindingID) String() string {
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

// Fact states that a subject holds a role inside a tenant.
type Fact struct {
	Subject   subject.Ref
	RoleName  string
	TenantID  string
	GrantedBy string
}

func NewFact(sub subject.Ref, roleName, tenantID, grantedBy string) (Fact, error) {
	roleName = strings.TrimSpace(roleName)
	tenantID = strings.TrimSpace(tenantID)
	grantedBy = strings.TrimSpace(grantedBy)
	if sub.IsZero() {
		return Fact{}, perrors.WithCode(code.ErrInvalidArgument, "subject is required")
	}
	if _, err := role.NewName(roleName); err != nil {
		return Fact{}, err
	}
	if _, err := tenant.NewID(tenantID); err != nil {
		return Fact{}, err
	}
	return Fact{Subject: sub, RoleName: roleName, TenantID: tenantID, GrantedBy: grantedBy}, nil
}
