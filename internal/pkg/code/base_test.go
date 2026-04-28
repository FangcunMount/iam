package code_test

import (
	"testing"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

func TestErrorCodeRegistration(t *testing.T) {
	// 创建一个带有 ErrBind 的错误
	err := errors.WithCode(code.ErrBind, "test error")

	// 解析错误码
	coder := errors.ParseCoder(err)

	// 检查 HTTP 状态码
	if coder.HTTPStatus() != 400 {
		t.Errorf("Expected HTTP status 400, got %d", coder.HTTPStatus())
	}

	// 检查错误码
	if coder.Code() != code.ErrBind {
		t.Errorf("Expected error code %d, got %d", code.ErrBind, coder.Code())
	}

	t.Logf("Error code: %d, HTTP status: %d, Message: %s",
		coder.Code(), coder.HTTPStatus(), coder.String())
}
