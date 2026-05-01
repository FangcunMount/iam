package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

type requiredPayload struct {
	Name string `json:"name" form:"name" binding:"required"`
}

func TestBaseHandlerBindJSONReturnsCodedBindError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewBaseHandler()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	var payload requiredPayload
	err := handler.BindJSON(c, &payload)

	requireBindError(t, err, w)
}

func TestBaseHandlerBindQueryReturnsCodedBindError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewBaseHandler()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

	var payload requiredPayload
	err := handler.BindQuery(c, &payload)

	requireBindError(t, err, w)
}

func requireBindError(t *testing.T, err error, w *httptest.ResponseRecorder) {
	t.Helper()

	require.Error(t, err)
	coder := perrors.ParseCoder(err)
	require.NotNil(t, coder)
	require.Equal(t, code.ErrBind, coder.Code())
	require.Equal(t, http.StatusBadRequest, w.Code)

	var response Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, code.ErrBind, response.Code)
}
