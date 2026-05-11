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
	TenantID    tenant.ID   // 租户ID（域）
	GrantedBy   string      // 授权人
}

// NewBinding 创建新赋权
func NewBinding(subjectType SubjectType, subjectID meta.ID, roleID meta.ID, tenantID string, opts ...BindingOption) (Binding, error) {
	subjectType = SubjectType(strings.TrimSpace(string(subjectType)))
	tenantIDValue, err := tenant.NewID(tenantID)
	if err != nil {
		return Binding{}, err
	}
	a := Binding{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		RoleID:      roleID,
		TenantID:    tenantIDValue,
	}
	for _, opt := range opts {
		opt(&a)
	}
	a.GrantedBy = strings.TrimSpace(a.GrantedBy)
	if _, err := subject.NewRef(subject.Type(subjectType), subjectID); err != nil {
		return Binding{}, err
	}
	if roleID.IsZero() {
		return Binding{}, perrors.WithCode(code.ErrInvalidArgument, "role id is required")
	}
	if a.GrantedBy == "" {
		return Binding{}, perrors.WithCode(code.ErrInvalidArgument, "granted by is required")
	}
	return a, nil
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
type SubjectType = subject.Type

const (
	SubjectTypeUser    = subject.TypeUser
	SubjectTypeGroup   = subject.TypeGroup
	SubjectTypeService = subject.TypeService
)

func (a Binding) SubjectTypeString() string {
	return string(a.SubjectType)
}

func (a Binding) TenantIDString() string {
	return a.TenantID.String()
}

func (a Binding) BelongsToTenant(tenantID string) bool {
	target, err := tenant.NewID(tenantID)
	if err != nil {
		return false
	}
	return a.TenantID == target
}

// Fact states that a subject holds a role inside a tenant.
type Fact struct {
	Subject   subject.Ref
	RoleName  role.Name
	TenantID  tenant.ID
	GrantedBy string
}

func NewFact(sub subject.Ref, roleName, tenantID, grantedBy string) (Fact, error) {
	grantedBy = strings.TrimSpace(grantedBy)
	if sub.IsZero() {
		return Fact{}, perrors.WithCode(code.ErrInvalidArgument, "subject is required")
	}
	roleNameValue, err := role.NewName(roleName)
	if err != nil {
		return Fact{}, err
	}
	tenantIDValue, err := tenant.NewID(tenantID)
	if err != nil {
		return Fact{}, err
	}
	return Fact{Subject: sub, RoleName: roleNameValue, TenantID: tenantIDValue, GrantedBy: grantedBy}, nil
}

func (f Fact) RoleNameString() string {
	return f.RoleName.String()
}

func (f Fact) TenantIDString() string {
	return f.TenantID.String()
}
