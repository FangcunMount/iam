// Package handler REST API 处理器基础
package handler

import (
	"net/http"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/gin-gonic/gin"

	"github.com/FangcunMount/iam/v3/internal/apiserver/transport/rest/authz/dto"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/FangcunMount/iam/v3/internal/pkg/requestctx"
	"github.com/FangcunMount/iam/v3/pkg/core"
)

// getTenantID 从上下文中获取租户ID。
func getTenantID(c *gin.Context) (string, error) {
	return requestctx.RequiredTenantID(c)
}

// getUserID 从上下文中获取用户ID。
func getUserID(c *gin.Context) (meta.ID, error) {
	return requestctx.RequiredUserID(c)
}

func bindJSON(c *gin.Context, req interface{}) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		handleError(c, perrors.WithCode(code.ErrBind, "请求参数错误: %v", err))
		return false
	}
	return true
}

func bindQuery(c *gin.Context, query interface{}) bool {
	if err := c.ShouldBindQuery(query); err != nil {
		handleError(c, perrors.WithCode(code.ErrBind, "请求参数错误: %v", err))
		return false
	}
	return true
}

func parseIDParam(c *gin.Context, name, message string) (meta.ID, bool) {
	id, err := meta.ParseID(c.Param(name))
	if err != nil {
		handleError(c, perrors.WithCode(code.ErrInvalidArgument, "%s", message))
		return 0, false
	}
	return id, true
}

// handleError 统一错误处理 (authz 模块特定的错误格式)
func handleError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	// 委托给 BaseHandler 的 Error 方法
	// 但是使用 authz 特定的错误响应格式
	core.NewBaseHandler().Error(c, err)
}

// success 成功响应 (authz 模块特定的响应格式)
func success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, dto.NewResponse(data))
}

// successList 分页列表成功响应 (authz 模块特定的响应格式)
func successList(c *gin.Context, data interface{}, total int64, offset, limit int) {
	c.JSON(http.StatusOK, dto.NewListResponse(data, total, offset, limit))
}

// successNoContent 无内容成功响应 (authz 模块特定的响应格式)
func successNoContent(c *gin.Context) {
	c.JSON(http.StatusOK, dto.Response{
		Code:    200,
		Message: "success",
	})
}
