package rolebinding

import (
	"context"

	bindingDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
)

// Commands 承载角色绑定写用例；REST 以 role_id 写入，gRPC 以 role_name 写入。
type Commands interface {
	Grant(ctx context.Context, cmd GrantCommand) (*bindingDomain.Binding, error)
	Revoke(ctx context.Context, cmd RevokeCommand) error
	RevokeByID(ctx context.Context, cmd RevokeByIDCommand) error
	NamedCommands
}

type NamedCommands interface {
	GrantByRoleName(ctx context.Context, cmd GrantByRoleNameCommand) error
	RevokeByRoleName(ctx context.Context, cmd RevokeByRoleNameCommand) error
}

// Directory 承载角色绑定读用例。
type Directory interface {
	ListBySubject(ctx context.Context, query ListBySubjectQuery) ([]*bindingDomain.Binding, error)
	ListByRole(ctx context.Context, query ListByRoleQuery) ([]*bindingDomain.Binding, error)
}

// GrantCommand 授权命令。
type GrantCommand struct {
	SubjectType bindingDomain.SubjectType
	SubjectID   string
	RoleID      uint64
	TenantID    string
	GrantedBy   string
}

// RevokeCommand 撤销授权命令。
type RevokeCommand struct {
	SubjectType bindingDomain.SubjectType
	SubjectID   string
	RoleID      uint64
	TenantID    string
}

// RevokeByIDCommand 根据 ID 撤销授权命令。
type RevokeByIDCommand struct {
	BindingID bindingDomain.BindingID
	TenantID  string
}

// ListBySubjectQuery 根据主体列出角色绑定查询。
type ListBySubjectQuery struct {
	SubjectType bindingDomain.SubjectType
	SubjectID   string
	TenantID    string
}

// ListByRoleQuery 根据角色列出角色绑定查询。
type ListByRoleQuery struct {
	RoleID   uint64
	TenantID string
}
