package code

import (
	"net/http"

	"github.com/FangcunMount/component-base/pkg/errors"
)

// IDP: 身份提供商相关错误码 (104000～104999).
const (
	// ErrWechatAppNotFound - 404: Wechat app not found.
	ErrWechatAppNotFound = 104000

	// ErrWechatAppAlreadyExists - 409: Wechat app already exists.
	ErrWechatAppAlreadyExists = 104001

	// ErrWechatAppTypeInvalid - 400: Wechat app type is invalid.
	ErrWechatAppTypeInvalid = 104002

	// ErrWechatAppStatusInvalid - 400: Wechat app status is invalid.
	ErrWechatAppStatusInvalid = 104003

	// ErrWechatAppTypeMismatch - 400: Wechat app type does not match the expected type for this flow.
	ErrWechatAppTypeMismatch = 104004
)

// nolint: gochecknoinits
func init() {
	registerIDP()
}

func registerIDP() {
	registerIDPCode(ErrWechatAppNotFound, http.StatusNotFound, "Wechat app not found")
	registerIDPCode(ErrWechatAppAlreadyExists, http.StatusConflict, "Wechat app already exists")
	registerIDPCode(ErrWechatAppTypeInvalid, http.StatusBadRequest, "Wechat app type is invalid")
	registerIDPCode(ErrWechatAppStatusInvalid, http.StatusBadRequest, "Wechat app status is invalid")
	registerIDPCode(ErrWechatAppTypeMismatch, http.StatusBadRequest, "Wechat app type does not match")
}

func registerIDPCode(code int, httpStatus int, message string) {
	errors.MustRegister(&idpCoder{
		code:   code,
		status: httpStatus,
		msg:    message,
	})
}

type idpCoder struct {
	code   int
	status int
	msg    string
}

func (c *idpCoder) Code() int {
	return c.code
}

func (c *idpCoder) HTTPStatus() int {
	return c.status
}

func (c *idpCoder) String() string {
	return c.msg
}

func (c *idpCoder) Reference() string {
	return ""
}
