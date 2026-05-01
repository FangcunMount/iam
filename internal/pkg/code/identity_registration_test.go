package code_test

import (
	"net/http"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/stretchr/testify/assert"
)

// TestIdentityErrorCodesRegistration 测试 identity.go 中的错误码是否正确注册
// 如果错误码未注册,ParseCoder 会返回 nil,导致无法获取 HTTP 状态码和错误消息
func TestIdentityErrorCodesRegistration(t *testing.T) {
	tests := []struct {
		name           string
		errorCode      int
		expectedStatus int
		shouldRegister bool
	}{
		{
			name:           "ErrUserNotFound",
			errorCode:      code.ErrUserNotFound,
			expectedStatus: http.StatusNotFound,
			shouldRegister: true,
		},
		{
			name:           "ErrUserAlreadyExists",
			errorCode:      code.ErrUserAlreadyExists,
			expectedStatus: http.StatusBadRequest,
			shouldRegister: true,
		},
		{
			name:           "ErrUserBasicInfoInvalid",
			errorCode:      code.ErrUserBasicInfoInvalid,
			expectedStatus: http.StatusBadRequest,
			shouldRegister: true,
		},
		{
			name:           "ErrUserStatusInvalid",
			errorCode:      code.ErrUserStatusInvalid,
			expectedStatus: http.StatusBadRequest,
			shouldRegister: true,
		},
		{
			name:           "ErrUserInvalid",
			errorCode:      code.ErrUserInvalid,
			expectedStatus: http.StatusBadRequest,
			shouldRegister: true,
		},
		{
			name:           "ErrUserBlocked",
			errorCode:      code.ErrUserBlocked,
			expectedStatus: http.StatusForbidden,
			shouldRegister: true,
		},
		{
			name:           "ErrUserInactive",
			errorCode:      code.ErrUserInactive,
			expectedStatus: http.StatusForbidden,
			shouldRegister: true,
		},
		{
			name:           "ErrIdentityProfileExists",
			errorCode:      code.ErrIdentityProfileExists,
			expectedStatus: http.StatusBadRequest,
			shouldRegister: true,
		},
		{
			name:           "ErrIdentityProfileNotFound",
			errorCode:      code.ErrIdentityProfileNotFound,
			expectedStatus: http.StatusNotFound,
			shouldRegister: true,
		},
		{
			name:           "ErrIdentityProfileLinkExists",
			errorCode:      code.ErrIdentityProfileLinkExists,
			expectedStatus: http.StatusBadRequest,
			shouldRegister: true,
		},
		{
			name:           "ErrIdentityProfileLinkNotFound",
			errorCode:      code.ErrIdentityProfileLinkNotFound,
			expectedStatus: http.StatusNotFound,
			shouldRegister: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 使用 WithCode 创建一个错误,然后解析它
			err := perrors.WithCode(tt.errorCode, "test error")
			coder := perrors.ParseCoder(err)

			if tt.shouldRegister {
				// 如果应该注册,检查是否能正确解析
				if coder == nil {
					t.Errorf("错误码 %d (%s) 未注册!ParseCoder 返回 nil", tt.errorCode, tt.name)
					return
				}

				// 验证 HTTP 状态码
				assert.Equal(t, tt.expectedStatus, coder.HTTPStatus(),
					"错误码 %s 的 HTTP 状态码不匹配", tt.name)

				// 验证错误码本身
				assert.Equal(t, tt.errorCode, coder.Code(),
					"错误码 %s 的代码值不匹配", tt.name)

				// 验证错误消息不为空
				assert.NotEmpty(t, coder.String(),
					"错误码 %s 的错误消息为空", tt.name)

				// 验证 IsCode 能正确识别
				assert.True(t, perrors.IsCode(err, tt.errorCode),
					"IsCode 应该能识别错误码 %s", tt.name)
			} else {
				// 如果不应该注册,应该返回 unknownCoder
				assert.NotNil(t, coder, "ParseCoder 不应该返回 nil")
				assert.NotEqual(t, tt.errorCode, coder.Code(),
					"未注册的错误码不应该有正确的 Code")
			}
		})
	}
}

// TestIdentityErrorCodesInUse 测试这些错误码是否能在实际使用中正常工作
func TestIdentityErrorCodesInUse(t *testing.T) {
	// 模拟使用 WithCode 创建错误
	err := perrors.WithCode(code.ErrUserNotFound, "user(123) not found")

	// 检查错误是否能正确识别
	assert.True(t, perrors.IsCode(err, code.ErrUserNotFound),
		"应该能识别 ErrUserNotFound 错误码")

	// 获取错误码对象
	coder := perrors.ParseCoder(err)
	if coder == nil {
		t.Fatal("ErrUserNotFound 未注册,ParseCoder 返回 nil")
	}

	// 验证错误码值
	assert.Equal(t, code.ErrUserNotFound, coder.Code(),
		"错误码值应该匹配")

	// 验证 HTTP 状态码
	assert.Equal(t, http.StatusNotFound, coder.HTTPStatus(),
		"ErrUserNotFound 应该返回 404 状态码")
}
