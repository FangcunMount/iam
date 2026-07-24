// Package safelog converts errors into bounded metadata suitable for logs.
// It intentionally never returns err.Error() or wrapped error arguments.
package safelog

import (
	"net/http"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
)

type ErrorDescriptor struct {
	Code       int
	HTTPStatus int
	Category   string
	Retryable  bool
}

func DescribeError(err error) ErrorDescriptor {
	if err == nil {
		return ErrorDescriptor{Category: "none"}
	}
	coder := perrors.ParseCoder(err)
	if coder == nil {
		return ErrorDescriptor{
			HTTPStatus: http.StatusInternalServerError,
			Category:   "internal",
			Retryable:  true,
		}
	}
	status := coder.HTTPStatus()
	return ErrorDescriptor{
		Code:       coder.Code(),
		HTTPStatus: status,
		Category:   category(status),
		Retryable:  status >= http.StatusInternalServerError,
	}
}

func category(status int) string {
	switch {
	case status == http.StatusUnauthorized:
		return "unauthenticated"
	case status == http.StatusForbidden:
		return "permission_denied"
	case status == http.StatusNotFound:
		return "not_found"
	case status == http.StatusConflict:
		return "conflict"
	case status >= http.StatusBadRequest && status < http.StatusInternalServerError:
		return "invalid_request"
	default:
		return "internal"
	}
}
