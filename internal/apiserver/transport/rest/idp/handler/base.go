// Package handler IDP 模块 REST API 处理器基础
package handler

import (
	"github.com/FangcunMount/iam/v3/pkg/core"
)

// BaseHandler 继承公共的 BaseHandler
type BaseHandler struct {
	*core.BaseHandler
}

// NewBaseHandler 创建基础 Handler
func NewBaseHandler() *BaseHandler {
	return &BaseHandler{
		BaseHandler: core.NewBaseHandler(),
	}
}
