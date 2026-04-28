package handler

import "github.com/FangcunMount/iam/pkg/core"

// BaseHandler 继承公共的基础响应能力。
type BaseHandler struct {
	*core.BaseHandler
}

// NewBaseHandler 创建基础处理器。
func NewBaseHandler() *BaseHandler {
	return &BaseHandler{
		BaseHandler: core.NewBaseHandler(),
	}
}
