package handler

import (
	"github.com/FangcunMount/iam/v2/pkg/core"
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
