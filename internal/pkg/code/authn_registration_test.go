package code_test

import (
	"net/http"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/stretchr/testify/assert"
)

// TestAuthnErrorCodesRegistration 测试 authn.go 中的错误码是否正确注册
func TestAuthnErrorCodesRegistration(t *testing.T) {
	// ErrTokenInvalid 应该已注册
	err := perrors.WithCode(code.ErrTokenInvalid, "token is invalid")
	coder := perrors.ParseCoder(err)

	assert.NotNil(t, coder, "ErrTokenInvalid 应该被注册")
	assert.Equal(t, code.ErrTokenInvalid, coder.Code(), "错误码应该匹配")
	assert.Equal(t, http.StatusUnauthorized, coder.HTTPStatus(), "应该返回 401")
	assert.True(t, perrors.IsCode(err, code.ErrTokenInvalid), "IsCode 应该能识别")
}

// TestAuthnErrorCodesUsage 测试 authn 错误码在实际场景中的使用
func TestAuthnErrorCodesUsage(t *testing.T) {
	tests := []struct {
		name           string
		errorCode      int
		expectedStatus int
		shouldRegister bool
	}{
		{
			name:           "ErrTokenInvalid",
			errorCode:      code.ErrTokenInvalid,
			expectedStatus: http.StatusUnauthorized,
			shouldRegister: true,
		},
		{
			name:           "ErrEncrypt",
			errorCode:      code.ErrEncrypt,
			expectedStatus: http.StatusUnauthorized,
			shouldRegister: true,
		},
		{
			name:           "ErrSignatureInvalid",
			errorCode:      code.ErrSignatureInvalid,
			expectedStatus: http.StatusUnauthorized,
			shouldRegister: true,
		},
		{
			name:           "ErrExpired",
			errorCode:      code.ErrExpired,
			expectedStatus: http.StatusUnauthorized,
			shouldRegister: true,
		},
		{
			name:           "ErrInvalidAuthHeader",
			errorCode:      code.ErrInvalidAuthHeader,
			expectedStatus: http.StatusUnauthorized,
			shouldRegister: true,
		},
		{
			name:           "ErrMissingHeader",
			errorCode:      code.ErrMissingHeader,
			expectedStatus: http.StatusUnauthorized,
			shouldRegister: true,
		},
		{
			name:           "ErrPasswordIncorrect",
			errorCode:      code.ErrPasswordIncorrect,
			expectedStatus: http.StatusUnauthorized,
			shouldRegister: true,
		},
		{
			name:           "ErrOTPSendTooFrequent",
			errorCode:      code.ErrOTPSendTooFrequent,
			expectedStatus: http.StatusTooManyRequests,
			shouldRegister: true,
		},
		{
			name:           "ErrUnsupportedAuthMethod",
			errorCode:      code.ErrUnsupportedAuthMethod,
			expectedStatus: http.StatusBadRequest,
			shouldRegister: true,
		},
		{
			name:           "ErrPayloadInvalid",
			errorCode:      code.ErrPayloadInvalid,
			expectedStatus: http.StatusBadRequest,
			shouldRegister: true,
		},
		{
			name:           "ErrProofBuildFailed",
			errorCode:      code.ErrProofBuildFailed,
			expectedStatus: http.StatusBadRequest,
			shouldRegister: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := perrors.WithCode(tt.errorCode, "test error")
			coder := perrors.ParseCoder(err)

			if tt.shouldRegister {
				assert.NotNil(t, coder, "%s 应该被注册", tt.name)
				assert.Equal(t, tt.errorCode, coder.Code(), "%s 错误码应该匹配", tt.name)
				assert.Equal(t, tt.expectedStatus, coder.HTTPStatus(), "%s HTTP状态码应该匹配", tt.name)
				assert.True(t, perrors.IsCode(err, tt.errorCode), "IsCode应该能识别 %s", tt.name)
			}
		})
	}
}
