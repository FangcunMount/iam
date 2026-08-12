package handler

import "github.com/FangcunMount/iam/v3/pkg/core"

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
