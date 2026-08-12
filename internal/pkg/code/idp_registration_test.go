package code_test

import (
	"net/http"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/assert"
)

func TestIDPErrorCodesRegistration(t *testing.T) {
	tests := []struct {
		name           string
		errorCode      int
		expectedStatus int
	}{
		{
			name:           "ErrWechatAppNotFound",
			errorCode:      code.ErrWechatAppNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "ErrWechatAppAlreadyExists",
			errorCode:      code.ErrWechatAppAlreadyExists,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "ErrWechatAppTypeInvalid",
			errorCode:      code.ErrWechatAppTypeInvalid,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "ErrWechatAppStatusInvalid",
			errorCode:      code.ErrWechatAppStatusInvalid,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := perrors.WithCode(tt.errorCode, "test error")
			coder := perrors.ParseCoder(err)

			if assert.NotNil(t, coder) {
				assert.Equal(t, tt.errorCode, coder.Code())
				assert.Equal(t, tt.expectedStatus, coder.HTTPStatus())
				assert.NotEmpty(t, coder.String())
			}
			assert.True(t, perrors.IsCode(err, tt.errorCode))
		})
	}
}
