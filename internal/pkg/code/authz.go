package code

import (
	"net/http"

	"github.com/FangcunMount/component-base/pkg/errors"
)

// Authz: 授权相关所有错误码 (103000～103999).

// Authz: 基础权限错误 (103000～103099).
const (
	// ErrUnauthorized - 403: Authorization failed.
	ErrUnauthorized = 103000

	// ErrPermissionDenied - 403: Permission denied.
	ErrPermissionDenied = 103001
)

// Authz: 角色相关错误 (103100～103199).
const (
	// ErrRoleNotFound - 404: Role not found.
	ErrRoleNotFound = 103100

	// ErrRoleAlreadyExists - 409: Role already exists.
	ErrRoleAlreadyExists = 103101

	// ErrRoleInheritanceAlreadyExists - 409: Role inheritance already exists.
	ErrRoleInheritanceAlreadyExists = 103102

	// ErrRoleInUse - 409: Role still has active authorization facts.
	ErrRoleInUse = 103103
)

// Authz: 资源相关错误 (103200～103299).
const (
	// ErrResourceNotFound - 404: Resource not found.
	ErrResourceNotFound = 103200

	// ErrResourceAlreadyExists - 409: Resource already exists.
	ErrResourceAlreadyExists = 103201

	// ErrInvalidAction - 400: Invalid action for resource.
	ErrInvalidAction = 103202

	// ErrResourceInUse - 409: Resource still has active permission grants.
	ErrResourceInUse = 103203
)

// Authz: 赋权相关错误 (103300～103399).
const (
	// ErrAssignmentNotFound - 404: Assignment not found.
	ErrAssignmentNotFound = 103300

	// ErrAssignmentAlreadyExists - 409: Assignment already exists.
	ErrAssignmentAlreadyExists = 103301
)

// Authz: 策略相关错误 (103400～103499).
const (
	// ErrPolicyVersionNotFound - 404: Policy version not found.
	ErrPolicyVersionNotFound = 103400
	// ErrPolicyVersionAlreadyExists - 409: Policy version already exists.
	ErrPolicyVersionAlreadyExists = 103401

	// ErrPermissionGrantAlreadyExists - 409: Permission grant already exists.
	ErrPermissionGrantAlreadyExists = 103402
)

// nolint: gochecknoinits
func init() {
	registerAuthz()
}

func registerAuthz() {
	// 基础权限错误
	registerAuthzCode(ErrUnauthorized, http.StatusForbidden, "Authorization failed")
	registerAuthzCode(ErrPermissionDenied, http.StatusForbidden, "Permission denied")

	// 角色相关错误
	registerAuthzCode(ErrRoleNotFound, http.StatusNotFound, "Role not found")
	registerAuthzCode(ErrRoleAlreadyExists, http.StatusConflict, "Role already exists")
	registerAuthzCode(ErrRoleInheritanceAlreadyExists, http.StatusConflict, "Role inheritance already exists")
	registerAuthzCode(ErrRoleInUse, http.StatusConflict, "Role is still in use")

	// 资源相关错误
	registerAuthzCode(ErrResourceNotFound, http.StatusNotFound, "Resource not found")
	registerAuthzCode(ErrResourceAlreadyExists, http.StatusConflict, "Resource already exists")
	registerAuthzCode(ErrInvalidAction, http.StatusBadRequest, "Invalid action for resource")
	registerAuthzCode(ErrResourceInUse, http.StatusConflict, "Resource is still in use")

	// 赋权相关错误
	registerAuthzCode(ErrAssignmentNotFound, http.StatusNotFound, "Assignment not found")
	registerAuthzCode(ErrAssignmentAlreadyExists, http.StatusConflict, "Assignment already exists")

	// 策略版本相关错误
	registerAuthzCode(ErrPolicyVersionNotFound, http.StatusNotFound, "Policy version not found")
	registerAuthzCode(ErrPolicyVersionAlreadyExists, http.StatusConflict, "Policy version already exists")
	registerAuthzCode(ErrPermissionGrantAlreadyExists, http.StatusConflict, "Permission grant already exists")

	// 策略相关错误
}

func registerAuthzCode(code int, httpStatus int, message string) {
	errors.MustRegister(&authzCoder{
		code:   code,
		status: httpStatus,
		msg:    message,
	})
}

// authzCoder 实现 errors.Coder 接口
type authzCoder struct {
	code   int
	status int
	msg    string
}

func (c *authzCoder) Code() int {
	return c.code
}

func (c *authzCoder) HTTPStatus() int {
	return c.status
}

func (c *authzCoder) String() string {
	return c.msg
}

func (c *authzCoder) Reference() string {
	return ""
}
